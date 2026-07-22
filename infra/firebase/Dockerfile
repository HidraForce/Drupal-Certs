FROM node:22-alpine
RUN npm install -g firebase-tools@14
WORKDIR /app
COPY firebase.json firestore.rules ./
EXPOSE 4000 8080 9099
CMD ["firebase", "emulators:start", "--project", "drupal-study-lab", "--import=/data", "--export-on-exit=/data"]
