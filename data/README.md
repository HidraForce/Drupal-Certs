# Question dataset contract

The production question bank intentionally lives outside the application services. Import or generate records matching `question.schema.json`, then replace the in-memory demo repository in `services/quiz` with your preferred source.

Never send `answer` or `explanation` from the list endpoint in production. The checking endpoint owns those fields.

The admin import screen assigns one certification track to the entire uploaded CSV: `frontend`, `backend`, or `devops`. The CSV keeps the columns shown in `questions.example.csv`; the selected track is stored with every imported Firestore document.
