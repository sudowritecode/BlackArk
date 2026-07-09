# BlackArk dashboard backend

Bun and Hono service for the BlackArk dashboard API.

## Local development

Install [Bun](https://bun.sh/), then run:

```sh
cd services/dashboard-backend
cp .env.example .env
bun install
bun run dev
```

The service listens on `http://localhost:3001` by default. Check it with:

```sh
curl http://localhost:3000/health
```

Environment variables are documented in `.env.example`. The defaults expect the
BlackArk control service at `http://localhost:8080` and the dashboard frontend at
`http://localhost:5173`.

## Checks and production build

```sh
bun test
bun run typecheck
bun run build
```

The build output is written to `dist/`. Run the built service with:

```sh
bun run dist/index.js
```
