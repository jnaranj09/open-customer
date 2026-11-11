---
paths:
  - 'web/src/lib/i18n/es.json'
  - 'internal/platform/problem/messages.go'
  - 'internal/locale/**'
---

# Rule: Spanish texts

Every text she reads in the app: `es.json`, the server messages in `messages.go`, and the export
names in `internal/locale`. Owner decisions from the reminders design
(`docs/designs/reminders-and-replies.md`, Detailed Implementation Plan, item 5).

- **A label beats a question; a name beats a sentence.** "Atributo fijo / Atributo por cita", not
  "¿Es igual siempre para la clienta, o cambia en cada cita?". Prose only.
- **Short.** Labels, chips and toasts about six words; a hint is one short line. Delete a hint that
  repeats its label. No explanatory paragraphs. Prose only.
- **Say the result, not the mechanism.** No "el sistema", "latido", "temporizador" or other
  technical words: "Sin conexión", "Tarea diaria". Prose only.
- **Never start with "Deja de…".** In Spanish it reads as an order to her ("¡Deja de crear
  mensajes!"). Say what will happen, in the future: "Ya no creará recordatorios", "Ya no aparecerá
  al agendar". Prose only.
- **One word per thing.** A **regla** is what she sets up in Ajustes; a **recordatorio** is one
  item to send in "Enviar"; a **mensaje** is the text sent. Never "recordatorio" for a rule.
  Prose only.
- **Items are shown, not "salen".** "No se mostrará de nuevo", not "No vuelve a salir". Prose only.
- **Agree in gender and number with the noun meant.** "Incluir atrasados" (recordatorios),
  "Ver archivadas" (clientas). Prose only.
- **Numbers and dates over words** in labels and toasts: "quedan 38", "15 nov". Prose only.
- **Address her with "tú"; the people she serves are "clientas".** Prose only.
