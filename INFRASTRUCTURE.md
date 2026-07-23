# Vercel + Firebase infrastructure setup

Production uses three Vercel projects connected to this one GitHub repository:

| Vercel project | Root Directory | Purpose |
|---|---|---|
| `drupal-certs-web` | `web` | Angular frontend |
| `drupal-certs-quiz` | `services/quiz` | Go/Chi quiz and admin API |
| `drupal-certs-progress` | `services/progress` | Go/Chi progress API |

The APIs run from `Dockerfile.vercel` container images. Firebase provides Google Authentication and Firestore. Every push to `main` is automatically deployed by Vercel's Git integration; `.github/workflows/ci.yml` separately verifies Go and Angular builds.

## 1. Firebase

1. Create a project in Firebase Console and register a Web app.
2. Enable **Authentication → Sign-in method → Google**.
3. Create Firestore in Native mode.
4. Download a service-account key from **Project settings → Service accounts → Firebase Admin SDK → Generate new private key**.
5. Never commit or add that JSON to the Angular project.
6. Deploy `infra/firebase/firestore.rules` with Firebase CLI.

## 2. Deploy the Quiz API to Vercel

1. In Vercel, choose **Add New → Project** and import this GitHub repository.
2. Name it `drupal-certs-quiz`.
3. Set **Root Directory** to `services/quiz`.
4. Vercel should detect `Dockerfile.vercel`. Do not use the Services preset and do not set npm build commands.
5. Add Production, Preview, and Development variables:

```env
FIREBASE_PROJECT_ID=your-firebase-project-id
FIREBASE_CREDENTIALS_JSON={the complete service-account JSON}
ADMIN_EMAILS=your-google-email@gmail.com
WEB_ORIGIN=https://your-future-web-project.vercel.app
```

6. Deploy and test `https://QUIZ-DOMAIN/health` and `https://QUIZ-DOMAIN/v1/questions`.

## 3. Deploy the Progress API to Vercel

1. Import the same repository as a second project.
2. Name it `drupal-certs-progress`.
3. Set **Root Directory** to `services/progress`.
4. Let Vercel detect `Dockerfile.vercel`.
5. Add:

```env
FIREBASE_PROJECT_ID=your-firebase-project-id
FIREBASE_CREDENTIALS_JSON={the complete service-account JSON}
WEB_ORIGIN=https://your-future-web-project.vercel.app
```

6. Deploy and test `https://PROGRESS-DOMAIN/health`. A request to `/v1/progress` without a Firebase token should return 401, which is correct.

## 4. Deploy Angular to Vercel

1. Import the same repository as a third project.
2. Name it `drupal-certs-web`.
3. Set **Root Directory** to `web`.
4. Vercel should detect Angular and read `web/vercel.json`:

```text
Install Command: npm ci
Build Command: npm run build
Output Directory: dist/web/browser
```

5. Add:

```env
FIREBASE_API_KEY=your-firebase-web-api-key
FIREBASE_AUTH_DOMAIN=your-project.firebaseapp.com
FIREBASE_PROJECT_ID=your-firebase-project-id
FIREBASE_APP_ID=your-firebase-web-app-id
QUIZ_API_URL=https://your-quiz-project.vercel.app
PROGRESS_API_URL=https://your-progress-project.vercel.app
```

6. Deploy and copy the stable production domain.

## 5. Connect the origins

On both API projects, replace `WEB_ORIGIN` with the exact Angular production origin:

```env
WEB_ORIGIN=https://your-web-project.vercel.app
```

Do not include a trailing slash. Redeploy both API projects after changing it. In Firebase Authentication → Settings → Authorized domains, add `your-web-project.vercel.app` without `https://`.

## 6. Admin CSV import

1. Ensure the quiz project has your exact verified Google email in `ADMIN_EMAILS`.
2. Sign into the Angular site using that account.
3. Open **Admin**, choose Front End, Back End, or DevOps for the ledger, and import `data/questions.example.csv`.

Imports are limited to 2 MiB and 400 rows. The chosen certification is stored on every imported question. Answers are zero-based: `0=A`, `1=B`, `2=C`, `3=D`. Re-importing an existing question ID updates it.

The Quiz API stores answered question IDs below each Firebase user and excludes them from all future selections. Three distinct user reports set `needsReview` on a question. Admin Review and Exhausted Users tabs read these Firestore records; no additional Vercel variables are required.

## Troubleshooting

- **`cd web: No such file or directory`**: the web project's root is already `web`; use the checked-in `web/vercel.json`, which does not run `cd web`.
- **API is detected as Angular or Node**: the API project's Root Directory is wrong. It must be `services/quiz` or `services/progress`.
- **Firebase default credentials error**: `FIREBASE_CREDENTIALS_JSON` is absent or malformed. Paste the complete JSON including the outer braces.
- **CORS error**: `WEB_ORIGIN` does not exactly equal the browser's Angular origin.
- **401 from `/v1/progress`**: expected without a Firebase bearer token.
- **403 from `/v1/admin/status`**: the signed-in verified email is not in `ADMIN_EMAILS`.
