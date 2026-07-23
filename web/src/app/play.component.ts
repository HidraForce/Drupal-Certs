import { HttpClient } from '@angular/common/http';
import { Component, OnDestroy, inject, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { APP_CONFIG } from './app.component';
import { AuthService } from './auth.service';

interface Question { id:string; certification:string; domain:string; prompt:string; options:string[] }
interface Result { correct:boolean; answer:number; explanation:string }

@Component({
  selector:'app-play', standalone:true, imports:[FormsModule,RouterLink],
  template:`<header class="nav"><a class="brand" routerLink="/home">Drupal<span>Forge</span> ⚙</a><a routerLink="/home">Leave chamber</a></header>
  <main class="game-page">
  @if(stage()==='setup'){
    <section class="setup-card framed"><span class="big-icon">⚙</span><span class="eyebrow">EXAMINATION CHAMBER</span><h1>Calibrate the machine</h1><p>Select the certification mechanism and the length of your trial. Each question allows thirty seconds.</p>
      <label>Certification track<select [(ngModel)]="certification"><option value="frontend">Front End Certification</option><option value="backend">Back End Certification</option><option value="devops">DevOps Certification</option></select></label>
      <label>Number of questions<select [(ngModel)]="count"><option [ngValue]="5">5 · Brief inspection</option><option [ngValue]="10">10 · Standard trial</option><option [ngValue]="20">20 · Master examination</option></select></label>
      <button class="primary wide" (click)="start()">Engage mechanism</button>
    </section>
  } @else if(stage()==='loading'){<section class="setup-card framed"><span class="big-icon pulse">⚙</span><h2>Pressurising question chamber…</h2></section>
  } @else if(stage()==='playing'&&current;as q){
    <section class="game-top"><div><span>{{trackLabel()}} · Question {{index()+1}} of {{questions().length}}</span><div class="progress"><i [style.width.%]="((index()+1)/questions().length)*100"></i></div></div><div class="timer" [class.danger]="seconds()<=10">◷ {{seconds()}}s</div></section>
    <section class="quiz-card framed"><div class="quiz-head"><span>{{q.domain}}</span><strong>Gauge {{score()}}</strong></div><h2>{{q.prompt}}</h2><div class="answers">@for(option of q.options;track option;let i=$index){<button [class.selected]="selected()===i" [class.correct]="result()&&result()?.answer===i" [class.wrong]="result()&&selected()===i&&!result()?.correct" (click)="choose(i)"><b>{{['A','B','C','D'][i]}}</b>{{option}}</button>}</div>@if(result();as f){<div class="feedback" [class.win]="f.correct"><span>{{f.correct?'✓':'!'}}</span><div><strong>{{f.correct?'Mechanism aligned':'Adjustment required'}}</strong><p>{{f.explanation}}</p></div></div>}<div class="actions">@if(!result()){<button class="primary" [disabled]="selected()===null" (click)="check()">Lock answer</button>}@else{<button class="primary" (click)="next()">{{index()+1===questions().length?'Read gauges':'Advance chamber'}}</button>}</div></section>
  } @else if(stage()==='done'){<section class="setup-card result-card framed"><span class="big-icon">{{score()/questions().length>=.7?'♛':'⚒'}}</span><span class="eyebrow">TRIAL COMPLETE</span><h1>{{score()}} / {{questions().length}}</h1><p>{{score()/questions().length>=.7?'The machine runs true. Fine craftsmanship.':'Return to the workshop and temper your knowledge.'}}</p><button class="primary wide" (click)="restart()">Recalibrate</button><a routerLink="/home">Return to workshop</a></section>
  } @else {<section class="setup-card framed"><h2>No {{trackLabel()}} questions found</h2><p>Import a ledger for this certification or select another track.</p><button class="primary" (click)="restart()">Recalibrate</button></section>}
  </main>`
})
export class PlayComponent implements OnDestroy {
  private http=inject(HttpClient);private config=inject(APP_CONFIG);private auth=inject(AuthService);
  stage=signal<'setup'|'loading'|'playing'|'done'|'error'>('setup');count=5;certification:'frontend'|'backend'|'devops'='frontend';
  questions=signal<Question[]>([]);index=signal(0);selected=signal<number|null>(null);result=signal<Result|null>(null);score=signal(0);seconds=signal(30);private timer?:number;
  get current(){return this.questions()[this.index()];}
  trackLabel(){return this.certification==='frontend'?'Front End':this.certification==='backend'?'Back End':'DevOps';}
  start(){this.stage.set('loading');this.http.get<Question[]>(`${this.config.quizApi}/v1/questions`).subscribe({next:all=>{const filtered=all.filter(q=>(q.certification||'frontend')===this.certification);this.questions.set([...filtered].sort(()=>Math.random()-.5).slice(0,Math.min(this.count,filtered.length)));if(!this.questions().length){this.stage.set('error');return;}this.stage.set('playing');this.startTimer();},error:()=>this.stage.set('error')});}
  choose(i:number){if(!this.result())this.selected.set(i);}
  async check(){const q=this.current,a=this.selected();if(!q||a===null)return;clearInterval(this.timer);const t=await this.auth.token();this.http.post<Result>(`${this.config.quizApi}/v1/questions/${q.id}/check`,{answer:a},{headers:{Authorization:`Bearer ${t}`}}).subscribe({next:r=>{this.result.set(r);if(r.correct)this.score.update(v=>v+1);},error:()=>this.next()});}
  next(){if(this.index()+1>=this.questions().length){void this.finish();return;}this.index.update(v=>v+1);this.selected.set(null);this.result.set(null);this.seconds.set(30);this.startTimer();}
  restart(){clearInterval(this.timer);this.stage.set('setup');this.index.set(0);this.score.set(0);this.selected.set(null);this.result.set(null);this.seconds.set(30);}
  ngOnDestroy(){clearInterval(this.timer);}
  private startTimer(){clearInterval(this.timer);this.timer=window.setInterval(()=>{this.seconds.update(v=>v-1);if(this.seconds()<=0){clearInterval(this.timer);this.next();}},1000);}
  private async finish(){clearInterval(this.timer);this.stage.set('done');try{const t=await this.auth.token();const h={headers:{Authorization:`Bearer ${t}`}};const p=await this.http.get<any>(`${this.config.progressApi}/v1/progress`,h).toPromise();await this.http.post(`${this.config.progressApi}/v1/progress`,{correct:(p?.correct||0)+this.score(),total:(p?.total||0)+this.questions().length,streak:Math.max(1,p?.streak||0)},h).toPromise();}catch{}}
}
