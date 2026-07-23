import { HttpClient } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { APP_CONFIG } from './app.component';
import { AuthService } from './auth.service';

interface ReviewItem { id:string; certification:string; domain:string; prompt:string; reportCount:number }
interface ExhaustionAlert { id:string; email:string; certification:string; answeredCount:number; availableCount:number }

@Component({
  selector:'app-admin', standalone:true, imports:[FormsModule,RouterLink],
  template:`<header class="nav"><a class="brand" routerLink="/home">Drupal<span>Forge</span> ⚙</a><a routerLink="/home">Return to workshop</a></header>
  <main class="admin-page">
  @if(checking()){<section class="setup-card"><span class="big-icon pulse">⚙</span><h2>Checking workshop credentials…</h2></section>}
  @else if(!allowed()){<section class="setup-card"><span class="big-icon">⛔</span><h2>Master’s credentials required</h2><p>Your email is not authorized by the Quiz API.</p><a class="primary" routerLink="/home">Return</a></section>}
  @else{
    <nav class="admin-tabs"><button [class.active]="tab()==='import'" (click)="tab.set('import')">Import ledger</button><button [class.active]="tab()==='review'" (click)="tab.set('review')">Review reports <b>{{reviews().length}}</b></button><button [class.active]="tab()==='alerts'" (click)="tab.set('alerts')">Exhausted users <b>{{alerts().length}}</b></button></nav>
    @if(tab()==='import'){
      <section class="admin-panel framed"><span class="eyebrow">QUESTION WORKSHOP</span><h1>Load a question ledger</h1><p>Assign this ledger to a certification mechanism. Every CSV row receives the selected track.</p>
        <label class="field-label">Certification exam<select [(ngModel)]="certification"><option value="frontend">Front End Certification</option><option value="backend">Back End Certification</option><option value="devops">DevOps Certification</option></select></label>
        <code class="columns">id, domain, prompt, option_a, option_b, option_c, option_d, answer, explanation, source</code>
        <label class="dropzone"><span class="gear-mark">⚙</span>@if(fileName()){<strong>{{fileName()}}</strong>}@else{<strong>Select CSV ledger</strong><span>2 MiB · maximum 400 questions</span>}<input type="file" accept=".csv,text/csv" (change)="select($event)"></label>
        @if(message()){<p class="form-success">{{message()}}</p>}@if(error()){<p class="form-error">{{error()}}</p>}
        <button class="primary wide" [disabled]="!file||importing()" (click)="upload()">{{importing()?'Loading ledger…':'Import into '+trackLabel()}}</button>
      </section>
    } @else if(tab()==='review'){
      <section class="admin-panel framed"><span class="eyebrow">INSPECTION BENCH</span><h1>Reported questions</h1><p>Questions appear here after reports from three distinct users.</p>
        <div class="ledger-list">@for(item of reviews();track item.id){<article><div><small>{{item.certification}} · {{item.domain}}</small><h3>{{item.prompt}}</h3><p>{{item.reportCount}} reports</p></div><button class="primary" (click)="resolveReview(item.id)">Mark reviewed</button></article>}@empty{<p class="empty-ledger">No mechanisms require inspection.</p>}</div>
      </section>
    } @else {
      <section class="admin-panel framed"><span class="eyebrow">SUPPLY WARNING</span><h1>Exhausted question banks</h1><p>These learners reached the end of an available certification bank without repeated questions.</p>
        <div class="ledger-list">@for(item of alerts();track item.id){<article><div><small>{{item.certification}}</small><h3>{{item.email||'Unknown learner'}}</h3><p>{{item.answeredCount}} answered · {{item.availableCount}} available in track</p></div><button class="primary" (click)="resolveAlert(item.id)">Acknowledge</button></article>}@empty{<p class="empty-ledger">All learners still have unanswered questions.</p>}</div>
      </section>
    }
  }</main>`
})
export class AdminComponent {
  private http=inject(HttpClient);private config=inject(APP_CONFIG);private auth=inject(AuthService);
  checking=signal(true);allowed=signal(false);importing=signal(false);fileName=signal('');message=signal('');error=signal('');tab=signal<'import'|'review'|'alerts'>('import');reviews=signal<ReviewItem[]>([]);alerts=signal<ExhaustionAlert[]>([]);
  certification:'frontend'|'backend'|'devops'='frontend';file:File|null=null;
  constructor(){void this.check();}
  trackLabel(){return this.certification==='frontend'?'Front End':this.certification==='backend'?'Back End':'DevOps';}
  select(event:Event){this.file=(event.target as HTMLInputElement).files?.[0]||null;this.fileName.set(this.file?.name||'');this.message.set('');this.error.set('');}
  async upload(){if(!this.file)return;this.importing.set(true);this.error.set('');const form=new FormData();form.append('file',this.file);form.append('certification',this.certification);try{const token=await this.auth.token();this.http.post<{imported:number}>(`${this.config.quizApi}/v1/admin/questions/import`,form,{headers:{Authorization:`Bearer ${token}`}}).subscribe({next:r=>{this.message.set(`Loaded ${r.imported} ${this.trackLabel()} questions.`);this.importing.set(false);},error:e=>{this.error.set(typeof e.error==='string'?e.error:'Import failed.');this.importing.set(false);}});}catch(e:any){this.error.set(e.message);this.importing.set(false);}}
  async resolveReview(id:string){const h=await this.headers();this.http.post(`${this.config.quizApi}/v1/admin/questions/${id}/review`,{},h).subscribe(()=>this.reviews.update(items=>items.filter(item=>item.id!==id)));}
  async resolveAlert(id:string){const h=await this.headers();this.http.post(`${this.config.quizApi}/v1/admin/exhaustion-alerts/${id}/resolve`,{},h).subscribe(()=>this.alerts.update(items=>items.filter(item=>item.id!==id)));}
  private async headers(){const token=await this.auth.token();return{headers:{Authorization:`Bearer ${token}`}};}
  private async check(){try{const h=await this.headers();this.http.get(`${this.config.quizApi}/v1/admin/status`,h).subscribe({next:()=>{this.allowed.set(true);this.checking.set(false);this.loadQueues(h);},error:()=>{this.allowed.set(false);this.checking.set(false);}});}catch{this.checking.set(false);}}
  private loadQueues(h:{headers:{Authorization:string}}){this.http.get<ReviewItem[]>(`${this.config.quizApi}/v1/admin/review`,h).subscribe(items=>this.reviews.set(items));this.http.get<ExhaustionAlert[]>(`${this.config.quizApi}/v1/admin/exhaustion-alerts`,h).subscribe(items=>this.alerts.set(items));}
}
