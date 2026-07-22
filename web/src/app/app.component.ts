import { HttpClient } from '@angular/common/http';
import { ChangeDetectionStrategy, Component, InjectionToken, inject, signal } from '@angular/core';
import { GoogleAuthProvider, User, getAuth, onAuthStateChanged, signInWithPopup, signOut } from 'firebase/auth';

export interface AppConfig { firebase: { apiKey: string; authDomain: string; projectId: string; appId: string }; quizApi: string; progressApi: string; useEmulators: boolean }
export const APP_CONFIG = new InjectionToken<AppConfig>('app.config');
interface Question { id: string; domain: string; prompt: string; options: string[] }
interface Result { correct: boolean; answer: number; explanation: string }

@Component({
  selector: 'app-root', standalone: true, changeDetection: ChangeDetectionStrategy.OnPush,
  templateUrl: './app.component.html', styleUrl: './app.component.css'
})
export class AppComponent {
  private readonly http = inject(HttpClient); private readonly config = inject(APP_CONFIG); private readonly auth = getAuth();
  readonly questions = signal<Question[]>([]); readonly index = signal(0); readonly selected = signal<number | null>(null); readonly result = signal<Result | null>(null); readonly score = signal(0); readonly user = signal<User | null>(null); readonly loading = signal(true);
  readonly isAdmin = signal(false); readonly adminOpen = signal(false); readonly importMessage = signal(''); readonly importing = signal(false); private csvFile: File | null = null;
  constructor() { onAuthStateChanged(this.auth, user => { this.user.set(user); this.isAdmin.set(false); if (user) this.checkAdmin(user); }); this.loadQuestions(); }
  get current(): Question | undefined { return this.questions()[this.index()]; }
  async login(): Promise<void> { await signInWithPopup(this.auth, new GoogleAuthProvider()); }
  async logout(): Promise<void> { await signOut(this.auth); }
  choose(answer: number): void { if (!this.result()) this.selected.set(answer); }
  check(): void { const q = this.current; const answer = this.selected(); if (!q || answer === null) return; this.http.post<Result>(`${this.config.quizApi}/v1/questions/${q.id}/check`, { answer }).subscribe(result => { this.result.set(result); if (result.correct) this.score.update(v => v + 1); }); }
  next(): void { if (this.index() < this.questions().length - 1) this.index.update(v => v + 1); else this.index.set(0); this.selected.set(null); this.result.set(null); }
  selectCSV(event: Event): void { this.csvFile = (event.target as HTMLInputElement).files?.[0] ?? null; this.importMessage.set(''); }
  async importCSV(): Promise<void> {
    const user = this.user(); if (!user || !this.csvFile) return; this.importing.set(true); this.importMessage.set('');
    const form = new FormData(); form.append('file', this.csvFile); const token = await user.getIdToken();
    this.http.post<{imported: number}>(`${this.config.quizApi}/v1/admin/questions/import`, form, { headers: { Authorization: `Bearer ${token}` } }).subscribe({ next: result => { this.importMessage.set(`Imported ${result.imported} questions ✨`); this.importing.set(false); this.loadQuestions(); }, error: error => { this.importMessage.set(error.error || 'Import failed. Check the CSV format.'); this.importing.set(false); } });
  }
  private loadQuestions(): void { this.loading.set(true); this.http.get<Question[]>(`${this.config.quizApi}/v1/questions`).subscribe({ next: q => { this.questions.set(q); this.index.set(0); this.loading.set(false); }, error: () => this.loading.set(false) }); }
  private async checkAdmin(user: User): Promise<void> { const token = await user.getIdToken(); this.http.get(`${this.config.quizApi}/v1/admin/status`, { headers: { Authorization: `Bearer ${token}` } }).subscribe({ next: () => this.isAdmin.set(true), error: () => this.isAdmin.set(false) }); }
}
