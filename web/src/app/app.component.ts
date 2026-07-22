import { ChangeDetectionStrategy, Component, InjectionToken } from '@angular/core';
import { RouterOutlet } from '@angular/router';
export interface AppConfig { firebase:{apiKey:string;authDomain:string;projectId:string;appId:string};quizApi:string;progressApi:string;useEmulators:boolean }
export const APP_CONFIG=new InjectionToken<AppConfig>('app.config');
@Component({selector:'app-root',standalone:true,imports:[RouterOutlet],changeDetection:ChangeDetectionStrategy.OnPush,template:'<router-outlet />'}) export class AppComponent{}
