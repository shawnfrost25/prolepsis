## DISCLAIMER
**Note:** This project is under active development. Core features are being implemented and tested rapidly, so APIs and functionality may break between commits as the architecture evolves.

## Current Features
* **Authentication & User Management:** Working registration, login, and user profile querying.
* **Token Validation:** Functioning session/token validation (currently backed by the primary database, with a planned migration to Redis).

## Note on Development
* **AI Usage:** AI was used only for a few low-value, repetitive tasks - mainly boilerplate and test files (e.g. the sqlc `test.sql` and Redis test files like `create_session.redis`). These are pure busywork and not worth writing by hand. It also helped with a couple of debugging sessions when the root cause wasn’t obvious, and with polishing comments and tone (this README being one example).
* **Author Implementation:** Every meaningful design decision, all core logic, and virtually all of the actual application code were thought through and written by the author. The AI’s role was limited to the minor, non-creative parts listed above.
