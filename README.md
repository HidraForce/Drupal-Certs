# Drupal Spark ✦

A lightweight, playful study app for Drupal front-end certification practice.

## Architecture

- `web`: standalone Angular SPA, CSS-only motion, Firebase Google sign-in
- `services/quiz`: public Go/Chi question and answer-checking API
- `services/progress`: authenticated Go/Chi progress API backed by Firestore
- `infra/firebase`: Dockerized Auth + Firestore emulators for local development
- `data`: separate question dataset contract (the checked-in questions are only UI demos)

For the complete Firebase, Railway/Cloud Run, Vercel, and admin setup, see [`INFRASTRUCTURE.md`](INFRASTRUCTURE.md).

## Run locally

Requirements: Docker, Go 1.24, Node 22.

```bash
docker compose up --build
cd web
npm install
USE_FIREBASE_EMULATORS=true npm start
```

Open `http://localhost:4200`; Firebase Emulator UI is at `http://localhost:4000`.

Google popup auth requires a real Firebase web app. Create one in Firebase Console, enable **Authentication → Google**, then put these values in `web/.env` or export them before `npm start`:

```bash
FIREBASE_API_KEY=...
FIREBASE_AUTH_DOMAIN=...
FIREBASE_PROJECT_ID=...
FIREBASE_APP_ID=...
```

For emulator-only work, create a test user through the Emulator UI. The app config is generated at build time and is intentionally ignored by Git.

## Deploy

### Railway APIs

Create one Railway project with services named `quiz` and `progress`, each rooted at its matching `services/*` directory. Set `WEB_ORIGIN` on both to the Vercel production URL. Set `FIREBASE_PROJECT_ID` and provide Firebase Admin credentials on both. Set `ADMIN_EMAILS` on the quiz service.

Add GitHub secrets `RAILWAY_QUIZ_TOKEN` and `RAILWAY_PROGRESS_TOKEN`. Railway can also auto-deploy the two service directories directly; if enabled, remove the `railway` job to avoid duplicate deployments.

### Vercel SPA

Import the repository into Vercel with the repository root (not `web`) as Root Directory. Add `FIREBASE_API_KEY`, `FIREBASE_AUTH_DOMAIN`, `FIREBASE_PROJECT_ID`, `FIREBASE_APP_ID`, `QUIZ_API_URL`, and `PROGRESS_API_URL` as production environment variables.

Add GitHub secrets `VERCEL_TOKEN`, `VERCEL_ORG_ID`, and `VERCEL_PROJECT_ID`. Every push to `main` runs tests, deploys both APIs, then deploys the SPA.

## Production notes

- Add the Vercel domain under Firebase Authentication → Authorized domains.
- Keep service-account credentials in Railway secrets; never prefix them with `NG_APP_` or ship them to Angular.
- The demo question repository exposes answers in Go memory only. Keep the external dataset private and return answers solely from the check endpoint.
