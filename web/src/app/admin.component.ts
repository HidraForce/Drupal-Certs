import { HttpClient } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { APP_CONFIG } from './app.component';
import { AuthService } from './auth.service';

@Component({
  selector: 'app-admin',
  standalone: true,
  imports: [FormsModule, RouterLink],
  template: `
    <header class="nav">
      <a class="brand" routerLink="/home">Drupal<span>Forge</span> ⚙</a>
      <a routerLink="/home">Return to workshop</a>
    </header>
    <main class="admin-page">
      @if (checking()) {
        <section class="setup-card"><span class="big-icon pulse">⚙</span><h2>Checking workshop credentials…</h2></section>
      } @else if (!allowed()) {
        <section class="setup-card"><span class="big-icon">⛔</span><h2>Master’s credentials required</h2><p>Your email is not authorized by the Quiz API.</p><a class="primary" routerLink="/home">Return</a></section>
      } @else {
        <section class="admin-panel framed">
          <span class="eyebrow">QUESTION WORKSHOP</span>
          <h1>Load a question ledger</h1>
          <p>First assign this ledger to a certification mechanism. Every row in the CSV will receive the selected track.</p>
          <label class="field-label">Certification exam
            <select [(ngModel)]="certification">
              <option value="frontend">Front End Certification</option>
              <option value="backend">Back End Certification</option>
              <option value="devops">DevOps Certification</option>
            </select>
          </label>
          <code class="columns">id, domain, prompt, option_a, option_b, option_c, option_d, answer, explanation, source</code>
          <label class="dropzone">
            <span class="gear-mark">⚙</span>
            @if (fileName()) { <strong>{{ fileName() }}</strong> } @else { <strong>Select CSV ledger</strong><span>2 MiB · maximum 400 questions</span> }
            <input type="file" accept=".csv,text/csv" (change)="select($event)">
          </label>
          @if (message()) { <p class="form-success">{{ message() }}</p> }
          @if (error()) { <p class="form-error">{{ error() }}</p> }
          <button class="primary wide" [disabled]="!file || importing()" (click)="upload()">{{ importing() ? 'Loading ledger…' : 'Import into ' + trackLabel() }}</button>
        </section>
      }
    </main>`
})
export class AdminComponent {
  private http = inject(HttpClient); private config = inject(APP_CONFIG); private auth = inject(AuthService);
  checking = signal(true); allowed = signal(false); importing = signal(false); fileName = signal(''); message = signal(''); error = signal('');
  certification: 'frontend'|'backend'|'devops' = 'frontend'; file: File|null = null;
  constructor(){ void this.check(); }
  trackLabel(){ return this.certification === 'frontend' ? 'Front End' : this.certification === 'backend' ? 'Back End' : 'DevOps'; }
  select(event: Event){ this.file=(event.target as HTMLInputElement).files?.[0]||null; this.fileName.set(this.file?.name||''); this.message.set(''); this.error.set(''); }
  async upload(){
    if(!this.file)return; this.importing.set(true); this.error.set('');
    const form=new FormData(); form.append('file',this.file); form.append('certification',this.certification);
    try{const token=await this.auth.token();this.http.post<{imported:number}>(`${this.config.quizApi}/v1/admin/questions/import`,form,{headers:{Authorization:`Bearer ${token}`}}).subscribe({next:r=>{this.message.set(`Loaded ${r.imported} ${this.trackLabel()} questions into the forge.`);this.importing.set(false);},error:e=>{this.error.set(typeof e.error==='string'?e.error:'Import failed. Inspect the ledger format.');this.importing.set(false);}});}catch(e:any){this.error.set(e.message);this.importing.set(false);}
  }
  private async check(){try{const token=await this.auth.token();this.http.get(`${this.config.quizApi}/v1/admin/status`,{headers:{Authorization:`Bearer ${token}`}}).subscribe({next:()=>{this.allowed.set(true);this.checking.set(false);},error:()=>{this.allowed.set(false);this.checking.set(false);}});}catch{this.checking.set(false);}}
}
