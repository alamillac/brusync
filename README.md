# Brusync (Go)

Servicio simple para Linux que revisa un directorio de forma recursiva cada cierto intervalo, y si detecta cambios los commitea y hace push a GitHub usando token.

Usa `inotify` (via `fsnotify`) para detectar cambios en tiempo real y agrupar eventos antes de sincronizar.

Pensado para mantener sincronizados los archivos generados por Bruno.

## Requisitos

- Go 1.22+
- Git instalado en el sistema
- Token Fine-grained de GitHub con acceso al repo objetivo

## Uso rapido

```bash
go run . \
  -dir "/ruta/a/tu/directorio/bruno" \
  -repo "https://github.com/tu-org/tu-repo.git" \
  -branch "main" \
  -interval 2m
```

El token se toma de `GITHUB_TOKEN` (recomendado) o `-token`.

```bash
export GITHUB_TOKEN="ghp_xxx"
export GITHUB_USER="tu-usuario-github"
```

## Flags

- `-dir`: directorio a sincronizar
- `-repo`: URL HTTPS del repositorio GitHub
- `-branch`: branch remota (default: `main`)
- `-interval`: cada cuanto revisar cambios (default: `5m`)
- `-debounce`: ventana para agrupar eventos de inotify (default: `2s`)
- `-token`: token GitHub (opcional si usas `GITHUB_TOKEN`)
- `-github-user`: usuario GitHub para autenticacion HTTPS (opcional, recomendado con PAT Fine-grained)
- `-commit-prefix`: prefijo del mensaje de commit
- `-author-name` / `-author-email`: autor de commits locales

## Build

```bash
go build -o brusync .
```

## Nota de seguridad

No pongas el token en la URL del `origin`. Esta app usa el token solo al momento de `push/pull`, y redacciona el token en mensajes de error.
