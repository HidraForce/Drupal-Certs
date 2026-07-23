# Drupal Spark ✦

A lightweight, playful study app for Drupal front-end certification practice.

Learners receive each question at most once per account and certification track. Distinct-user reports move a question into the admin review queue after three reports, and exhausted question banks create admin supply alerts.

## Architecture

- `web`: standalone Angular SPA, CSS-only motion, Firebase Google sign-in
- `services/quiz`: public Go/Chi question and answer-checking API deployed as a Vercel container
- `services/progress`: authenticated Go/Chi progress API backed by Firestore and deployed as a Vercel container
- `infra/firebase`: Dockerized Auth + Firestore emulators for local development
- `data`: separate question dataset contract (the checked-in questions are only UI demos)

For the complete Firebase, Vercel API/frontend, and admin setup, see [`INFRASTRUCTURE.md`](INFRASTRUCTURE.md).

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

Create three Vercel projects from the same repository with roots `web`, `services/quiz`, and `services/progress`. Vercel deploys the Angular project normally and both Go APIs from their `Dockerfile.vercel` files. The complete variable list and click-by-click instructions are in [`INFRASTRUCTURE.md`](INFRASTRUCTURE.md).

## Production notes

- Add the Vercel domain under Firebase Authentication → Authorized domains.
- Keep service-account credentials only in the two Vercel API projects; never ship them to Angular.
- The demo question repository exposes answers in Go memory only. Keep the external dataset private and return answers solely from the check endpoint.
