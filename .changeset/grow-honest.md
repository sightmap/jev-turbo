---
"@sightmap/jev-turbo": patch
---

grow decides button, link, input and select from roles and tags, and asks Jev only about repeated containers (nav, card, or noise). `grow.Namer` is an interface with `TemplateNamer` as the default, the place where a curator that proposes names can plug in. `bench --grow` writes the grown components and model calls into the run file. grow is coverage scaffolding: it scopes each new component to the nearest stable ancestor the page already has, verifies the selector, names it from a template, reads off `label`, `href` and `placeholder` as properties, and names views from their routes. Any property beyond those three, a memory line, or a name a person would choose still comes from the authoring skill or a curator.
