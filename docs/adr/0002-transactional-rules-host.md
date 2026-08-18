# ADR 0002: Host transaccional persistente de reglas

- Estado: Aceptado
- Fecha: 2026-08-18
- Depende de: ADR 0001

## Decisión

El estado de sesión persiste, junto al lock exacto y al estado opaco del
ruleset, los datos necesarios para ejecutar una resolución mecánica de forma
reintentable y auditable:

- `InitialState` y batches ordenados de eventos para reconstruir el estado;
- recibos con ID, tool, fingerprint SHA-256 y resultado exacto;
- `PendingStep`, petición pública, revisión y principal iniciador para continuar
  decisiones o adjudicaciones después de guardar y reabrir;
- draws genéricos con método, fuente, especificación, resultado, step, request,
  lock y principal;
- revisión mecánica y generación del host como contadores separados;
- una cadena causal SHA-256 que atestigua el runtime completo de cada generación.

`Revision` avanza una vez por cada batch que el ruleset procesa con `Reduce`.
Un commit que solo añade un recibo, un draw o una continuación no inventa una
revisión mecánica, pero sí avanza `Generation`, necesaria para fusionar cambios
de un subprocesso MCP aunque el ruleset sea stateless.
La generación por sí sola no prueba ascendencia: almacenamiento e importación
aceptan una generación posterior únicamente si su cadena causal contiene como
prefijo exacto la historia ya persistida. Dos procesos que parten del mismo
snapshot no pueden sobrescribirse sólo porque uno haya acumulado más commits.

## Transacción

Una llamada mutante reclama primero su ID en `SessionState`. Dos routers que
reciben simultáneamente el mismo ID y fingerprint comparten el recibo final; no
repiten azar ni efectos. El mismo ID con otra tool o argumentos se rechaza.
Distintos IDs ejecutan sobre snapshots optimistas y el commit compara tanto la
revisión como la generación exactas. El gateway serializa los intents que aún
podrían consumir entropía; las respuestas concurrentes compiten mediante CAS.

El ruleset se ejecuta fuera del mutex de sesión. El host atiende `NeedRandom` a
través de un dispatcher extensible por `RandomRequest.Method`, y `Emit` solo a
través de `Reduce` seguido de `ValidateState`. Cada resultado de azar se guarda
junto al `PendingStep` y su `HostResponse` **antes** de llamar `Resume`; un batch
reducido se guarda con el nuevo estado y su acuse de recibo antes de reanudar.
Tras una caída, el host reanuda ese checkpoint y nunca vuelve a sortear o reducir.
Al terminar o suspenderse, estado, batches, draws, pendiente, recibo y proyección
al log legacy se validan sobre una copia y se intercambian bajo un único mutex.
Un error de validación o conflicto no puede dejar una mutación mecánica parcial.

Los errores de ejecución también producen un recibo acotado para que un retry
del proveedor reproduzca el error. Si un proveedor de entropía ya devolvió un
resultado válido a nivel de protocolo, el draw permanece auditado aunque el
ruleset rechace después su semántica; nunca se vuelve a sortear.

## Recibos y pendientes

Se conservan hasta 4096 recibos y hasta 64 MiB agregados. No se expulsa ni se
olvida ningún ID aceptado: alcanzar cualquiera de los límites rechaza IDs nuevos
antes de ejecutar reglas o azar, mientras los retries retenidos siguen
funcionando. Cada solicitud reserva antes de ejecutar el peor tamaño permitido
para su resultado; las reservas concurrentes también cuentan contra el límite,
por lo que un commit nunca descubre falta de espacio después de producir un
efecto. Una futura compactación requerirá un checkpoint/epoch explícito.
Hay un máximo de 64 resoluciones pendientes; alcanzar el límite rechaza el
commit completo en vez de perder una continuación activa.

`game_submit_intent` devuelve `needs_input` para `NeedDecision` y
`NeedAdjudication`. `game_respond` recupera el `PendingStep`, comprueba revisión y
autoridad, y llama `Resume` con el principal actual autorizado. La continuación
opaca y los datos de azar nunca se exponen al LLM. `game_observe` permite volver
a descubrir las peticiones pendientes que el principal actual está autorizado a
contestar, sin publicar continuaciones ni respuestas del host.

## Replay y auditoría

`ruleshost.Replay` parte de `InitialState` en revisión cero, exige secuencia
contigua y que cada batch avance exactamente una revisión, vuelve a llamar
`Reduce`, valida cada estado intermedio y devuelve el snapshot reconstruido. El
consumidor compara ese snapshot con el estado materializado para detectar
alteraciones.

El audit atribuye cada batch y draw al request, resolución, principal y lock
exacto. Los resultados del azar son payloads opacos: dados, cartas, tablas y
otros métodos pueden registrarse sin introducir conceptos de un sistema
concreto en el host.

## Compatibilidad y límites

`roll_dice` y `ability_check` atraviesan la misma transacción y conservan su
texto y entrada de log anteriores. Los bloques de sesión creados antes de este
ADR, que no tienen eventos, usan su estado actual como raíz inicial al cargarse.
Las herramientas MCP mutantes de compatibilidad usan los mismos recibos
durables, incluso cuando no invocan un ruleset, para que un reintento tras una
respuesta perdida no vuelva a cambiar la ficha o el mundo.

Cada commit invoca, antes de reanudar o devolver, la barrera opcional
`Session.PersistRules`. Los frontends que prometan durabilidad frente a caída la
conectan a su reemplazo atómico del archivo; si queda nil se conserva el contrato
histórico de memoria más autosave, sin afirmar durabilidad de proceso.
La aplicación de escritorio, el servicio HTTP, el bot y el subprocesso MCP ya
conectan esta barrera al archivo canónico de sesión; MCP actualiza además su
copia temporal antes de contestar a la llamada.
El namespace global de IDs por instancia/conexión se resuelve en los adapters de
Oracle y MCP, no en el host. Las resoluciones hijas requieren el catálogo
multi-ruleset y no se presentan como una decisión humana.
