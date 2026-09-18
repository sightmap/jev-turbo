---
"@sightmap/jev-turbo": patch
---

The "back" action is offered only after the run has navigated at least once. From the first page, `history.back()` leaves the site for the tab's blank page and the loop cannot recover. The demo renderer puts the page in a browser frame with the run's actions, sightmap components, and finish check beside it.
