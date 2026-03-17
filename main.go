package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type config struct {
	Dir          string
	RepoURL      string
	Branch       string
	Interval     time.Duration
	Token        string
	GitHubUser   string
	CommitPrefix string
	AuthorName   string
	AuthorEmail  string
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := parseConfig()
	if err != nil {
		slog.Error("configuracion invalida", "error", err)
		os.Exit(1)
	}

	if err := ensureGitAvailable(); err != nil {
		slog.Error("git no disponible", "error", err)
		os.Exit(1)
	}

	if err := validateRepoAccess(cfg); err != nil {
		slog.Error("fallo validacion de acceso al repositorio", "repo", cfg.RepoURL, "error", err)
		os.Exit(1)
	}

	if err := setupRepository(cfg); err != nil {
		slog.Error("no se pudo preparar el repositorio", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := syncOnce(ctx, cfg); err != nil {
		slog.Error("sincronizacion inicial con error", "error", err)
	}

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	slog.Info("iniciando observacion", "dir", cfg.Dir, "interval", cfg.Interval.String())

	for {
		select {
		case <-ctx.Done():
			slog.Info("detenido por senal")
			return
		case <-ticker.C:
			if err := syncOnce(ctx, cfg); err != nil {
				slog.Error("error en sincronizacion", "error", err)
			}
		}
	}
}

func parseConfig() (config, error) {
	var cfg config

	flag.StringVar(&cfg.Dir, "dir", ".", "Directorio a sincronizar (recursivo)")
	flag.StringVar(&cfg.RepoURL, "repo", "", "URL HTTPS del repo GitHub (ej: https://github.com/org/repo.git)")
	flag.StringVar(&cfg.Branch, "branch", "main", "Branch remota a mantener sincronizada")
	flag.DurationVar(&cfg.Interval, "interval", 5*time.Minute, "Intervalo de revision (ej: 2m, 30s)")
	flag.StringVar(&cfg.Token, "token", "", "Token GitHub (opcional, tambien GITHUB_TOKEN)")
	flag.StringVar(&cfg.GitHubUser, "github-user", "", "Usuario GitHub para autenticacion HTTPS (opcional, tambien GITHUB_USER)")
	flag.StringVar(&cfg.CommitPrefix, "commit-prefix", "chore(sync): update Bruno data", "Prefijo del mensaje de commit")
	flag.StringVar(&cfg.AuthorName, "author-name", "Brusync Bot", "Nombre autor de commits locales")
	flag.StringVar(&cfg.AuthorEmail, "author-email", "brusync-bot@local", "Email autor de commits locales")
	flag.Parse()

	if cfg.Token == "" {
		cfg.Token = os.Getenv("GITHUB_TOKEN")
	}
	if cfg.GitHubUser == "" {
		cfg.GitHubUser = os.Getenv("GITHUB_USER")
	}

	if cfg.RepoURL == "" {
		return cfg, errors.New("falta -repo")
	}
	if cfg.Token == "" {
		return cfg, errors.New("falta token: usa -token o GITHUB_TOKEN")
	}
	if cfg.Interval < 10*time.Second {
		return cfg, errors.New("-interval debe ser >= 10s")
	}

	absDir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return cfg, fmt.Errorf("directorio invalido: %w", err)
	}
	cfg.Dir = absDir

	st, err := os.Stat(cfg.Dir)
	if err != nil {
		return cfg, fmt.Errorf("no se pudo acceder al directorio: %w", err)
	}
	if !st.IsDir() {
		return cfg, errors.New("-dir debe ser un directorio")
	}

	return cfg, nil
}

func ensureGitAvailable() error {
	_, err := runGit(context.Background(), ".", "", "--version")
	if err != nil {
		return fmt.Errorf("git no esta disponible en PATH")
	}
	return nil
}

func setupRepository(cfg config) error {
	ctx := context.Background()

	if _, err := runGit(ctx, cfg.Dir, cfg.Token, "rev-parse", "--is-inside-work-tree"); err != nil {
		if _, err := runGit(ctx, cfg.Dir, cfg.Token, "init"); err != nil {
			return err
		}
	}

	if _, err := runGit(ctx, cfg.Dir, cfg.Token, "config", "user.name", cfg.AuthorName); err != nil {
		return err
	}
	if _, err := runGit(ctx, cfg.Dir, cfg.Token, "config", "user.email", cfg.AuthorEmail); err != nil {
		return err
	}

	if _, err := runGit(ctx, cfg.Dir, cfg.Token, "remote", "get-url", "origin"); err != nil {
		if _, err := runGit(ctx, cfg.Dir, cfg.Token, "remote", "add", "origin", cfg.RepoURL); err != nil {
			return err
		}
	} else {
		if _, err := runGit(ctx, cfg.Dir, cfg.Token, "remote", "set-url", "origin", cfg.RepoURL); err != nil {
			return err
		}
	}

	if _, err := runGit(ctx, cfg.Dir, cfg.Token, "show-ref", "--verify", "--quiet", "refs/heads/"+cfg.Branch); err != nil {
		if _, createErr := runGit(ctx, cfg.Dir, cfg.Token, "checkout", "-b", cfg.Branch); createErr != nil {
			if _, checkoutErr := runGit(ctx, cfg.Dir, cfg.Token, "checkout", cfg.Branch); checkoutErr != nil {
				return fmt.Errorf("no se pudo crear/usar branch %s: %w", cfg.Branch, createErr)
			}
		}
	} else {
		if _, err := runGit(ctx, cfg.Dir, cfg.Token, "checkout", cfg.Branch); err != nil {
			return err
		}
	}

	return nil
}

func validateRepoAccess(cfg config) error {
	ctx := context.Background()
	authURLs, err := authURLs(cfg.RepoURL, cfg.Token, cfg.GitHubUser)
	if err != nil {
		return err
	}

	var lastErr error
	for _, authURL := range authURLs {
		if _, err := runGit(ctx, cfg.Dir, cfg.Token, "ls-remote", "--exit-code", authURL, "HEAD"); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	return fmt.Errorf("no se pudo leer el repo remoto con el token (repo inexistente, token sin permisos o usuario incorrecto): %w", lastErr)
}

func syncOnce(ctx context.Context, cfg config) error {
	needsPush, err := hasPendingPush(ctx, cfg)
	if err != nil {
		return err
	}
	if needsPush {
		slog.Info("hay commits pendientes; intentando push")
		if err := pushWithRetry(ctx, cfg); err != nil {
			return err
		}
	}

	if _, err := runGit(ctx, cfg.Dir, cfg.Token, "add", "-A"); err != nil {
		return err
	}

	status, err := runGit(ctx, cfg.Dir, cfg.Token, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(status) == "" {
		slog.Info("sin cambios")
		return nil
	}

	msg := fmt.Sprintf("%s (%s)", cfg.CommitPrefix, time.Now().Format(time.RFC3339))
	if _, err := runGit(ctx, cfg.Dir, cfg.Token, "commit", "-m", msg); err != nil {
		return err
	}

	if err := pushWithRetry(ctx, cfg); err != nil {
		return err
	}

	slog.Info("cambios sincronizados")
	return nil
}

func hasPendingPush(ctx context.Context, cfg config) (bool, error) {
	localHead, err := runGit(ctx, cfg.Dir, cfg.Token, "rev-parse", "HEAD")
	if err != nil {
		if strings.Contains(err.Error(), "unknown revision") || strings.Contains(err.Error(), "ambiguous argument 'HEAD'") {
			return false, nil
		}
		return false, err
	}
	localHead = strings.TrimSpace(localHead)

	authURLs, err := authURLs(cfg.RepoURL, cfg.Token, cfg.GitHubUser)
	if err != nil {
		return false, err
	}

	var lastErr error
	for _, authURL := range authURLs {
		remoteLine, lsErr := runGit(ctx, cfg.Dir, cfg.Token, "ls-remote", authURL, "refs/heads/"+cfg.Branch)
		if lsErr != nil {
			lastErr = lsErr
			continue
		}

		remoteLine = strings.TrimSpace(remoteLine)
		if remoteLine == "" {
			return true, nil
		}

		remoteFields := strings.Fields(remoteLine)
		if len(remoteFields) == 0 {
			return true, nil
		}

		remoteHead := remoteFields[0]
		return localHead != remoteHead, nil
	}

	if lastErr != nil {
		return false, lastErr
	}

	return false, nil
}

func pushWithRetry(ctx context.Context, cfg config) error {
	authURLs, err := authURLs(cfg.RepoURL, cfg.Token, cfg.GitHubUser)
	if err != nil {
		return err
	}

	pushRef := fmt.Sprintf("HEAD:refs/heads/%s", cfg.Branch)
	var lastErr error
	for _, authURL := range authURLs {
		if _, err := runGit(ctx, cfg.Dir, cfg.Token, "push", authURL, pushRef); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	slog.Warn("push rechazado por cambios remotos; intentando pull --rebase", "error", lastErr)
	for _, authURL := range authURLs {
		if _, err := runGit(ctx, cfg.Dir, cfg.Token, "pull", "--rebase", authURL, cfg.Branch); err != nil {
			lastErr = err
			continue
		}
		if _, err := runGit(ctx, cfg.Dir, cfg.Token, "push", authURL, pushRef); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	return lastErr
}

func authURLs(repoURL, token, githubUser string) ([]string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return nil, fmt.Errorf("repo URL invalida: %w", err)
	}
	if u.Scheme != "https" {
		return nil, errors.New("solo se soporta URL https")
	}

	urls := make([]string, 0, 2)
	if githubUser != "" {
		userURL := *u
		userURL.User = url.UserPassword(githubUser, token)
		urls = append(urls, userURL.String())
	}

	appURL := *u
	appURL.User = url.UserPassword("x-access-token", token)
	urls = append(urls, appURL.String())

	return urls, nil
}

func runGit(parent context.Context, dir, token string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()

	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	out, err := cmd.CombinedOutput()
	outStr := redactToken(string(out), token)
	argsStr := redactToken(strings.Join(args, " "), token)

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("git %s excedio timeout", argsStr)
		}
		return "", fmt.Errorf("git %s fallo: %v: %s", argsStr, err, strings.TrimSpace(outStr))
	}

	return outStr, nil
}

func redactToken(input, token string) string {
	if token == "" {
		return input
	}
	return strings.ReplaceAll(input, token, "***")
}
