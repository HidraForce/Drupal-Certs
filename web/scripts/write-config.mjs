import { mkdirSync, writeFileSync } from 'node:fs';

const config = {
  firebase: {
    apiKey: process.env.FIREBASE_API_KEY || 'demo-key',
    authDomain: process.env.FIREBASE_AUTH_DOMAIN || 'drupal-study-lab.firebaseapp.com',
    projectId: process.env.FIREBASE_PROJECT_ID || 'drupal-study-lab',
    appId: process.env.FIREBASE_APP_ID || 'demo-app-id'
  },
  quizApi: process.env.QUIZ_API_URL || 'http://localhost:8081',
  progressApi: process.env.PROGRESS_API_URL || 'http://localhost:8082',
  useEmulators: process.env.USE_FIREBASE_EMULATORS === 'true'
};
mkdirSync('public', { recursive: true });
writeFileSync('public/app-config.json', JSON.stringify(config));
