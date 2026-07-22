import 'zone.js';
import { bootstrapApplication } from '@angular/platform-browser';
import { provideHttpClient } from '@angular/common/http';
import { provideRouter } from '@angular/router';
import { initializeApp } from 'firebase/app';
import { connectAuthEmulator, getAuth } from 'firebase/auth';
import { AppComponent, APP_CONFIG, AppConfig } from './app/app.component';
import { routes } from './app/app.routes';

fetch('/app-config.json', { cache: 'no-store' })
  .then(response => { if (!response.ok) throw new Error(`Configuration returned ${response.status}`); return response.json() as Promise<AppConfig>; })
  .then(config => {
    const firebaseApp = initializeApp(config.firebase);
    const auth = getAuth(firebaseApp);
    if (config.useEmulators) connectAuthEmulator(auth, 'http://localhost:9099', { disableWarnings: true });
    return bootstrapApplication(AppComponent, { providers: [provideHttpClient(), provideRouter(routes), { provide: APP_CONFIG, useValue: config }] });
  })
  .catch(error => { console.error('Could not start Drupal Spark', error); document.body.innerHTML = `<main style="font-family:system-ui;padding:3rem;max-width:700px;margin:auto"><h1>Drupal Spark could not start</h1><p>The deployment configuration could not be loaded. Check that the Vercel project root is <b>web</b> and redeploy.</p><pre>${String(error).replaceAll('<','&lt;')}</pre></main>`; });
