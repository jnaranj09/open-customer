# SAD — open-customer

open-customer es un sistema self-hosted de clientas, citas y recordatorios por WhatsApp para un
emprendimiento de servicios. Es un solo binario Go que sirve la API y la PWA, con PostgreSQL.
La instalación de referencia corre en una PC de la casa con Docker Compose y cuatro contenedores
siempre activos (`app`, `db`, `cloudflared`, `backup`), detrás de un Cloudflare Tunnel, sin puertos
publicados. El detalle de la arquitectura vive en un solo lugar:
[docs/designs/release-1.md](designs/release-1.md).

El SAD original, con el texto completo de los ADR 001–017, está en
[designs/history/sad-original.md](designs/history/sad-original.md).

## Índice de ADRs

| ADR | Título | Estado en release 1 | Detalle |
| --- | --- | --- | --- |
| 001 | Backend en Go | Se mantiene | [Vista del sistema](designs/release-1.md#system-view) |
| 002 | PostgreSQL | Se mantiene; la versión la fija el 025 | [Modelo de datos](designs/release-1.md#data-model) |
| 003 | `pgx` v5 + `sqlc` | Se mantiene | [Módulos](designs/release-1.md#code-layout-and-module-rules) |
| 004 | OpenAPI spec-first con `oapi-codegen` | Cambia: OpenAPI 3.0.3 en vez de 3.1 | [API](designs/release-1.md#api) |
| 005 | Monolito modular | Cambia: un solo proceso `serve`, sin modo `worker` aparte | [Procesos](designs/release-1.md#process-model) |
| 006 | Cola de trabajos en Postgres con `SKIP LOCKED` | Reemplazado por el 019 | — |
| 007 | Migraciones `goose` embebidas al arrancar | Se mantiene; con la guardia de respaldo del 022 | [Upgrades y guardia de migración](designs/release-1.md#upgrades-and-the-migration-guard) |
| 008 | SvelteKit `adapter-static` embebido con `go:embed`, como PWA | Se mantiene; estilos por el 023 | [Módulos](designs/release-1.md#code-layout-and-module-rules) |
| 009 / 015 | PWA instalada en vez de app nativa de iOS | Se mantiene | [Alternativa B1](designs/release-1.md#alternative-b1-native-ios-app-swiftui) |
| 010 | Fotos: volumen local, S3 opcional | Cambia: solo local, S3 diferido | [Fotos](designs/release-1.md#photos) |
| 011 | Reverse proxy Caddy con Let's Encrypt | Reemplazado por el 018 | — |
| 012 | Sesiones opacas en Postgres + Argon2id, en una cookie | Se mantiene | [Auth y sesiones](designs/release-1.md#auth-sessions-and-handover) |
| 013 | Renderer de templates propio, tipo Mustache | Se mantiene; el guardado de templates por el 020 | [Plantillas](designs/release-1.md#message-templates) |
| 014 | Licencia (MIT o AGPL-3.0) | Pendiente | [Non-Goals](designs/release-1.md#non-goals) |
| 016 | Web Push con VAPID | Cambia: una notificación diaria con el resumen | [Push diario](designs/release-1.md#daily-push) |
| 017 | IA on-device opcional | Diferido | [Mejoras futuras](designs/release-1.md#future-improvements) |
| 018 | Ingress con Cloudflare Tunnel | Nuevo | [Despliegue](designs/release-1.md#deployment) |
| 019 | Lista de recordatorios calculada al leer + bitácora de recordatorios | Nuevo | [Cálculo de la lista](designs/release-1.md#due-list-computation) |
| 020 | Biblioteca de plantillas de mensaje | Nuevo | [Plantillas](designs/release-1.md#message-templates) |
| 021 | Offline de solo lectura + refresco por ventana de fechas | Nuevo | [Cache y offline](designs/release-1.md#phone-cache-and-offline-behavior) |
| 022 | Respaldos: un contenedor, restic, destino USB, guardia de migración | Nuevo | [Respaldos](designs/release-1.md#backups) |
| 023 | Tailwind CSS v4 + componentes propios | Nuevo | [Alternativa M3](designs/release-1.md#alternative-m3-shadcn-svelte-tailwind--bits-ui) |
| 024 | Login con contraseña en la PWA instalada | Nuevo | [Auth y sesiones](designs/release-1.md#auth-sessions-and-handover) |
| 025 | PostgreSQL 18 | Nuevo | [Alternativa H2](designs/release-1.md#alternative-h2-postgresql-17-or-19) |

Los ADR 018–025 no tienen archivo propio: su detalle es la sección enlazada. El título del 022
es el original; hoy el destino es cualquier carpeta que se elige al instalar (`BACKUP_TARGET`), y un
USB o segundo disco es opcional.

**Regla:** un cambio de diseño edita este archivo solo cuando agrega un ADR o cambia el estado de
uno. Todo lo demás se cambia en `docs/designs/release-1.md`.
