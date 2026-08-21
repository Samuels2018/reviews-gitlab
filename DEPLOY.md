# Despliegue de kc-review-bot en Render.com + cron-job.org

El repositorio incluye un pequeño servidor HTTP en Go (`cmd/server`) que ejecuta el
comando `kc-review-bot` cada vez que recibe una petición `GET /run`.

## Cómo funciona

| Endpoint   | Descripción |
|------------|-------------|
| `GET /health` | Devuelve `{"status":"ok"}`. Sirve como keep-alive. |
| `GET /run`    | Ejecuta `kc-review-bot`. Protegido con token. |

Autenticación de `/run` (si defines `RUN_SECRET`, cualquiera de estas formas vale):

- Cabecera: `Authorization: Bearer <RUN_SECRET>`
- Cabecera: `X-Run-Token: <RUN_SECRET>`
- Query param: `/run?key=<RUN_SECRET>`

Parámetro opcional `?async=1`: responde `202` al instante y ejecuta el bot en segundo
plano. Útil porque el plan gratuito de cron-job.org corta las peticiones a los ~30 s y
una ejecución contra GitLab puede tardar más.

## Variables de entorno

| Variable | Obligatoria | Descripción |
|---------------|----|-------------|
| `GITLAB_HOST` | sí | `gitlab.com` o tu GitLab self-hosted (p. ej. `https://gitlab.tiendasdaka.com`) |
| `GITLAB_REPO` | sí | `owner/repo` o ID numérico del proyecto |
| `GITLAB_TOKEN` | recomendado | Token de API de GitLab con permiso `read_api` |
| `WEBHOOK_URL` | recomendado | Webhook de Slack para enviar el recordatorio |
| `CHANNEL` | no | Canal (`MiCanal`) o usuario (`@usuario`) destino |
| `WEBHOOK_AUTH` | no | Cabecera Authorization del webhook, si la requiere |
| `RUN_SECRET` | recomendado | Token secreto del endpoint `/run` (Render puede generarlo) |
| `REVIEWERS_PATH` | no | Ruta al `reviewers.json` (por defecto `/app/reviewers.json`, incluido en la imagen) |
| `TEMPLATE_PATH` | no | Plantilla Go alternativa |
| `RUN_TIMEOUT` | no | Segundos máximos por ejecución (por defecto 300) |

Nunca pongas tokens en el código: se configuran en el dashboard de Render.

## Paso 1 · Subir a GitHub

El `reviewers.json` contiene emails reales: usa un repositorio **privado**.

```bash
git add Dockerfile render.yaml cmd/server DEPLOY.md main.go main_test.go \
        common hoster/gitlab/gitlab.go hoster/gitlab/gitlab_test.go \
        hoster/gitlab/client_moq_test.go reviewers.json
git commit -m "Add HTTP server for Render deployment"
git push origin main
```

## Paso 2 y 3 · Crear el servicio en Render

1. Entra en <https://dashboard.render.com> → **New** → **Web Service**.
2. Conecta tu cuenta de GitHub y selecciona el repo `kc-review-bot`
   (Render tiene acceso también a repos privados).
3. Si usas el Blueprint: detectará `render.yaml` automáticamente. Si lo creas a mano:
   - **Runtime**: Docker
   - **Plan**: Free
   - **Health Check Path**: `/health`
4. Añade las variables de entorno de la tabla anterior. Para `RUN_SECRET` puedes usar
   *Generate value*.
5. **Create Web Service** y espera el deploy. Apunta la URL (p. ej. `https://kc-review-bot.onrender.com`).

Prueba manual:

```bash
curl -H "Authorization: Bearer $RUN_SECRET" https://tu-servicio.onrender.com/run
```

## Paso 4 · Programar cron-job.org

1. Crea una cuenta en <https://cron-job.org> y pulsa **Cronjobs → Create cronjob**.
2. Configuración:
   - **URL**: `https://tu-servicio.onrender.com/run?key=<RUN_SECRET>&async=1`
   - **Schedule**: cada día a la hora deseada (p. ej. 09:00), zona horaria `Europe/Madrid`.
3. Guarda y activa el job.

### Recomendación: keep-alive

Las instancias gratuitas de Render se duermen tras ~15 min de inactividad y el arranque
en frío puede tardar ~1 minuto. Para evitarlo, crea un segundo cronjob en cron-job.org
que llame cada 10 minutos a `https://tu-servicio.onrender.com/health`. Mantener una sola
instancia encendida 24/7 cabe dentro de las 750 h/mes gratuitas de Render.

## Estructura añadida

```
cmd/server/main.go   Servidor HTTP (/run, /health) que ejecuta el binario del bot
Dockerfile           Build multi-stage: compila bot + servidor e instala reviewers.json
render.yaml          Blueprint de Render (runtime docker, plan free)
reviewers.json       Revisadores (formatos aceptados: mapa {email, slack_id}, mapa legacy {"user": "@handle"} y array [{username, discordId, labels}])
```

Notas:

- Un reviewer sin `labels` se considera relevante para todos los MR; si le pones
  `labels`, solo se le recordará en MR que tengan alguna de esas etiquetas.
- El `slack_id` se convierte automáticamente en mención de Slack (`<@U...>`).
- El bot solo soporta GitLab por ahora (la ruta GitHub está sin implementar).
