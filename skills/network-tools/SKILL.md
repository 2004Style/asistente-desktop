---
name: network-tools
description: Diagnóstico de red local, DNS, puertos propios, conectividad y servicios sin realizar acciones ofensivas.
version: 1.0.0
author: RBot Max Pack
risk_level: medium
voice_triggers:
  - "revisa mi red"
  - "no tengo internet"
  - "revisa dns"
  - "qué ip tengo"
  - "qué puertos tengo abiertos"
  - "revisa el puerto"
  - "ping"
permissions:
  - exec:ip
  - exec:ping
  - exec:ss
  - exec:dig
  - exec:resolvectl
  - exec:curl
---

# Skill: Network Tools

Diagnostica red y servicios locales.

## Permitido

- Revisar IP local.
- Revisar puerta de enlace.
- Probar DNS.
- Probar conectividad.
- Ver puertos locales abiertos.
- Revisar servicios locales del usuario.

## No permitido por defecto

- Escanear redes ajenas.
- Explotar servicios.
- Fuerza bruta.
- Ataques MITM.
- Captura de credenciales.

## Comandos

```bash
ip a
ip route
ping -c 3 1.1.1.1
ping -c 3 google.com
resolvectl status
ss -tulpn
curl -I <url>
dig <dominio>
```

## Reglas

1. Si se pide escaneo, limitar a máquinas propias/laboratorio autorizado.
2. Explicar si una acción requiere autorización.
3. Para puertos, usar `ss` local antes que escaneos externos.

## Ejemplos

- "qué puertos tengo abiertos" → `ss -tulpn`.
- "no tengo internet" → ping IP, ping dominio, DNS.
- "revisa mi dominio" → DNS/curl.
