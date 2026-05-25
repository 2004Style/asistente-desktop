---
name: git-guardian
description: Operaciones Git seguras: estado, ramas, commits, diffs, logs y protección ante push/reset/clean destructivo.
version: 2.0.0
author: RBot Max Pack
risk_level: high
voice_triggers:
  - "git status"
  - "estado de git"
  - "crea una rama"
  - "crea un branch"
  - "haz commit"
  - "muestra el diff"
  - "qué cambios tengo"
  - "sube los cambios"
  - "haz push"
permissions:
  - exec:git
---

# Skill: Git Guardian

Gestiona Git con seguridad.

## Acciones seguras

```bash
git status --short
git status
git branch --show-current
git branch
git diff --stat
git diff
git log --oneline -n 10
```

## Acciones medias

Crear rama:

```bash
git checkout -b <nombre>
```

Commit:

```bash
git add <archivos>
git commit -m "<mensaje>"
```

Antes de commit:

1. Ejecutar `git status --short`.
2. Mostrar resumen.
3. Confirmar mensaje si no está claro.

## Acciones de alto riesgo

Requieren confirmación fuerte:

```bash
git reset --hard
git clean -fd
git push --force
git rebase
git checkout -- <archivo>
```

## Reglas

1. Nunca hacer `git commit -am` si hay archivos nuevos sin trackear importantes.
2. Antes de push, mostrar rama actual y remoto.
3. No empujar a `main` o `master` sin confirmación.
4. Si hay conflictos, no resolver automáticamente salvo instrucción clara.

## Ejemplos

- "qué cambios tengo" → status + diff stat.
- "crea la rama feat-skills" → checkout -b.
- "haz un commit que diga mejora skills" → status, add, commit.
