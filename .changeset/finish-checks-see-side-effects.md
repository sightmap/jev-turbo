---
"@sightmap/jev-turbo": patch
---

A finish check can now say what must not be on the page. `text_absent` holds when the page does not show a text, and `text_count` bounds how often a text shows: exactly N times, at least N, or between N and M (`--done-when text_count=1:BILLY`, `1+:BILLY`, `1-2:BILLY`). A presence check could only assert that the right item was in the bag; a run that added it twice passed `text_contains` and scored as a success. A count sees the second row. Occurrences are counted on the innermost nodes that show the text, since a node's rendered text is its whole subtree's, so a product name is one per bag row, not one per level of nesting.
