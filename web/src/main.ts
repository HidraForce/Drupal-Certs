import { bootstrapApplication } from '@angular/platform-browser';
import { provideHttpClient } from '@angular/common/http';
import { initializeApp } from 'firebase/app';
import { connectAuthEmulator, getAuth } from 'firebase/auth';
import { AppComponent, APP_CONFIG, AppConfig } from './app/app.component';

fetch('/app-config.json')
  .then(response => response.json() as Promise<AppConfig>)
  .then(config => {
    const firebaseApp = initializeApp(config.firebase);
    const auth = getAuth(firebaseApp);
    if (config.useEmulators) connectAuthEmulator(auth, 'http://localhost:9099', { disableWarnings: true });
    return bootstrapApplication(AppComponent, { providers: [provideHttpClient(), { provide: APP_CONFIG, useValue: config }] });
  })
  .catch(error => console.error('Could not start Drupal Spark', error));
