# Infrastructure setup

This project supports two backend targets:

1. **Railway** — simplest deployment and the existing CI workflow target.
2. **Google Cloud Run** — closest to “hosting the Go API on Firebase.” Firebase Hosting cannot execute Go itself; Cloud Run executes each container inside the same Google Cloud project used by Firebase.

## 1. Create Firebase resources

1. Create a project at Firebase Console and record its project ID.
2. Add a **Web app** under Project settings.
3. In Authentication → Sign-in method, enable **Google**.
4. Create Firestore in Native mode. Choose the region nearest most users; changing it later is difficult.
5. In Authentication → Settings → Authorized domains, later add the Vercel production domain.
6. Deploy the checked-in rules from `infra/firebase`:

```bash
firebase login
firebase use --add YOUR_PROJECT_ID
firebase deploy --only firestore:rules
```

The Go services use the Firebase Admin SDK and therefore need Application Default Credentials. On Cloud Run this is automatic. On Railway, create a narrowly scoped Google Cloud service account, download its JSON key once, store the JSON as a Railway secret, and never commit it.

## 2A. Deploy the APIs to Railway

Create one Railway project and two services from the GitHub repository:

| Service | Root directory | Variables |
|---|---|---|
| `quiz` | `/services/quiz` | `FIREBASE_PROJECT_ID`, `WEB_ORIGIN`, `ADMIN_EMAILS`, credentials |
| `progress` | `/services/progress` | `FIREBASE_PROJECT_ID`, `WEB_ORIGIN`, credentials |

`WEB_ORIGIN` must exactly match `https://YOUR-SITE.vercel.app` with no trailing slash. `ADMIN_EMAILS` is a comma-separated allowlist of Google-account emails. Tokens must contain a verified email, and the backend enforces the list.

For credentials, store the service-account JSON in a secret variable and expose it as a file or use Railway's documented sealed-variable/file mechanism. Point `GOOGLE_APPLICATION_CREDENTIALS` at that file. Do not place service credentials in Vercel.

Generate a Railway project token for each service, then create GitHub secrets:

- `RAILWAY_QUIZ_TOKEN`
- `RAILWAY_PROGRESS_TOKEN`

## 2B. Deploy the APIs to Cloud Run

Cloud Run has free usage quota but requires a billing account and upgrades the Firebase project to Blaze. Set a small budget alert before deploying.

```bash
gcloud auth login
gcloud config set project YOUR_PROJECT_ID
gcloud services enable run.googleapis.com cloudbuild.googleapis.com artifactregistry.googleapis.com

gcloud run deploy drupal-quiz \
  --source services/quiz \
  --region southamerica-east1 \
  --allow-unauthenticated \
  --min-instances 0 \
  --set-env-vars FIREBASE_PROJECT_ID=YOUR_PROJECT_ID,WEB_ORIGIN=https://YOUR-SITE.vercel.app,ADMIN_EMAILS=you@example.com

gcloud run deploy drupal-progress \
  --source services/progress \
  --region southamerica-east1 \
  --allow-unauthenticated \
  --min-instances 0 \
  --set-env-vars FIREBASE_PROJECT_ID=YOUR_PROJECT_ID,WEB_ORIGIN=https://YOUR-SITE.vercel.app
```

“Allow unauthenticated” permits the browser to reach the services. Protected routes still verify Firebase ID tokens in application code. Leave minimum instances at zero to scale to zero.

Grant each Cloud Run runtime service account only the Firebase/Firestore permissions it needs. Record both generated service URLs for Vercel.

## 3. Deploy Angular to Vercel

The repository-level `vercel.json` already sets the install command, build command, output directory, and SPA fallback.

1. Push the repository to GitHub.
2. In Vercel, select **Add New → Project** and import the repository.
3. Keep **Root Directory** at the repository root because `vercel.json` is there.
4. Add these Production and Preview environment variables:

| Variable | Value |
|---|---|
| `FIREBASE_API_KEY` | Firebase web-app config |
| `FIREBASE_AUTH_DOMAIN` | `YOUR_PROJECT_ID.firebaseapp.com` |
| `FIREBASE_PROJECT_ID` | Firebase project ID |
| `FIREBASE_APP_ID` | Firebase web-app config |
| `QUIZ_API_URL` | Railway or Cloud Run quiz URL |
| `PROGRESS_API_URL` | Railway or Cloud Run progress URL |

5. Deploy, copy the final `*.vercel.app` domain, add it to Firebase Authorized domains, and set both APIs' `WEB_ORIGIN` to that exact URL. Redeploy the APIs after changing it.

The Firebase API key is intentionally present in browser configuration; it identifies the Firebase project and is not an Admin credential. Security comes from Firebase Authentication, Firestore Rules, server-side token verification, and API-key restrictions configured in Google Cloud.

## 4. Use the admin panel

1. Set your Google email in the quiz service's `ADMIN_EMAILS` variable.
2. Sign into the deployed Angular app using that exact Google account.
3. The **Admin** button appears after the backend validates the token.
4. Import `data/questions.example.csv` or a file following `data/question.schema.json`.

Imports are limited to 2 MiB and 400 rows. IDs are Firestore document IDs, so importing an existing ID updates that question. Correct answers are zero-based: `0=A`, `1=B`, `2=C`, `3=D`.

## 5. Production checklist

- Set a Google Cloud billing budget and alerts; alerts notify but do not cap spend.
- Restrict CORS using the exact Vercel origin.
- Use separate Firebase projects for development and production.
- Rotate any downloaded service-account key and prefer keyless workload identity when the host supports it.
- Confirm `/health` on both APIs, Google sign-in, admin CSV import, a quiz answer, and progress saving.
