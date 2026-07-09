# BlackArk dashboard

Vue 3, Vite, and TypeScript single-page application for the BlackArk dashboard.

## Local development

Install Node.js 20.19+ or 22.12+, then run:

```sh
cd web/dashboard
npm install
npm run dev
```

The dashboard is available at `http://localhost:5173`. During development, Vite
proxies `/api` and `/health` requests to the Hono backend at
`http://localhost:3000`. Start that service separately from
`services/dashboard-backend`.

## Checks and production build

```sh
npm run typecheck
npm run build
```

The production build is written to `dist/`. Preview it locally with
`npm run preview`.
