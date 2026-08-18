# ADR-0001: Kernel de rulesets y paquetes versionados

- Estado: Aceptado
- Fecha: 2026-08-18

## Contexto

thAImaturgy separa la aventura inmutable del estado mutable de sesión, pero su
mecánica está acoplada hoy a D&D 5e:

- `Adventure.System` es una etiqueta y no selecciona comportamiento.
- `engine.ToolRouter` publica tools estáticas y las despacha con un `switch`.
- `ability_check`, `Character`, `StatBlock`, descansos, conjuros e
  `internal/srd` contienen semántica 5e.
- los prompts base hablan explícitamente de D&D.
- `SessionState.Log` es una cronología humana, no un event log mecánico.

El objetivo es ejecutar D&D, Pathfinder, RuneQuest, Call of Cthulhu, Vampire,
Shadowrun, juegos PbtA, GURPS, Fate, Savage Worlds y sistemas futuros sin dejar
las reglas autoritativas en manos del LLM.

El repositorio ya ofrece dos fronteras aprovechables: las tools de
`internal/types` no dependen del proveedor de LLM y `mcptools.ToolProvider`
publica cualquier catálogo/ejecutor compatible por MCP.

## Decisión

Adoptamos un **kernel pequeño** que ejecuta **paquetes de ruleset versionados**.
El contrato host-paquete usa datos serializables y no depende de Go, Starlark,
Lua ni WASM.

```text
LLM · DM · jugadores · clientes
              │
        tools `game_*`
              │
         Tool gateway
    world · session · rules
              │
          Rules host
 registry · schemas · transactions
 events/RNG · permissions · projections
              │
 ruleset fijado por id/version/digest/protocol
```

### Familias, rulesets y contenido

Una **familia mecánica** es una técnica reutilizable: d20, d100, pool, 2d6,
3d6, dados explosivos o cartas. No es un sistema instalable. D&D y Pathfinder
comparten d20 pero no críticos, grados, acciones ni personajes; RuneQuest y CoC
comparten porcentajes pero no reglas; PbtA es una familia cuyos movimientos
dependen de cada juego.

Un **ruleset** implementa un juego y edición concretos (`dnd5e`, `pf2e`,
`coc7e`, etc.). Define acciones legales, estado mecánico, resoluciones, efectos,
proyecciones y explicaciones.

Un **paquete de contenido** contiene criaturas, objetos, conjuros o playbooks y
declara su dependencia del ruleset. Una **aventura** contiene mundo, escenas,
PNJ, secretos y assets; declara un requisito de ruleset, pero no ejecuta código.

### Frontera kernel-paquete

El paquete recibe snapshots inmutables y devuelve outcomes, eventos y efectos
propuestos. Nunca recibe un puntero mutable a `SessionState`. Todo runtime adapta
estas operaciones lógicas:

```text
describe()                          -> metadatos y capacidades
catalog(snapshot, principal)        -> acciones aplicables
start(intent, snapshot, context)    -> paso de resolución
resume(pending, response, snapshot) -> siguiente paso
project(snapshot, principal)        -> vista autorizada
explain(rule_ref, locale, snapshot, principal) -> explicación/referencias
validate(snapshot)                  -> invariantes
migrate(from_version, snapshot)     -> migración explícita
```

El host conserva autenticación, permisos, revisiones, idempotencia, RNG,
persistencia, límites y commits atómicos. El paquete conserva la semántica del
juego. `project` es la única vista entregada a LLM o clientes.

### Tools estables

El gateway publica inicialmente:

| Tool | Función |
| --- | --- |
| `game_observe` | Proyección autorizada y resoluciones pendientes |
| `game_list_actions` | Acciones aplicables ahora |
| `game_get_action_schema` | Schema de una acción concreta |
| `game_submit_intent` | Iniciar una resolución tipada |
| `game_respond` | Responder a elección, reacción o adjudicación |
| `game_preview` | Consultar legalidad/posibles efectos sin azar ni mutación |
| `game_explain` | Explicar un resultado o referencia visible |

Las tools pertenecen al kernel; acciones como `attack`, `basic_move` o
`invoke_aspect` pertenecen al paquete. Así, instalar un sistema no multiplica la
superficie publicada a cada proveedor.

`catalog` devuelve cada acción junto con su schema. Por tanto,
`game_get_action_schema` es una proyección del catálogo del gateway, no una
operación adicional del ruleset ni una segunda fuente de verdad.

`game_preview` también se deriva en el gateway: ejecuta el `start` puro solo
hasta el primer `Step` y no atiende azar, hijos o emisiones ni confirma eventos.
Informa de legalidad y de la siguiente frontera observable; no promete enumerar
todos los resultados futuros ni actúa como oráculo de probabilidades.

El gateway inyecta `request_id`, sesión, principal, roles y
`expected_revision`; no confía esos datos al modelo. Repetir un `request_id`
devuelve el mismo resultado y no duplica eventos ni consume azar de nuevo.
Las tools mecánicas 5e actuales serán aliases temporales durante la migración.

### Resolución suspendible

Internamente, `start` y `resume` devuelven exactamente una variante `Step`:

- `reject`;
- `need_random`;
- `need_decision`;
- `need_adjudication`;
- `start_child`;
- `emit`;
- `complete`.

El host atiende automáticamente azar, resoluciones hijas y emisiones. Para los
pasos no terminales persiste un `PendingStep` con ID, tipo y continuación opaca;
la respuesta del host repite ID y tipo para impedir que se aplique a otra
resolución. La API `game_*` proyecta esas variantes internas a estos estados:

- `resolved`: finalizado;
- `needs_input`: espera elección, reacción, gasto, adjudicación o azar físico;
- `rejected`: intención ilegal, sin efectos;
- `conflict`: la revisión quedó obsoleta y hay que volver a observar.

Una resolución pendiente serializa `resolution_id`, continuación opaca y
acotada, revisión y autoridad esperada. No conserva closures ni una VM viva, por
lo que sobrevive a autosave y reinicio.

Cada paso es una transacción corta: el host obtiene snapshot/revisión; el
paquete valida y propone; el host valida schemas e invariantes; finalmente
confirma eventos y continuación de forma atómica. Nunca se mantiene una
transacción abierta esperando al LLM o a una persona. Un error, panic o timeout
no confirma cambios parciales.

El outcome común no impone vocabulario d20:

```json
{
  "status": "needs_input",
  "resolution_id": "res_992",
  "outcome": {
    "code": "fate.attack.succeeds_with_style",
    "traits": {"success": true},
    "system_data": {"shifts": 3}
  },
  "requests": [{
    "id": "choose_benefit",
    "authority": "player",
    "options": ["create_boost", "extra_shift"]
  }],
  "rule_refs": ["fate.attack.outcomes"]
}
```

`success`, `critical`, `margin` o `degree` son traits opcionales; la fuente de
verdad es `outcome.code` con `system_data` validado. La misma máquina representa
reacciones d20, éxitos parciales PbtA, invocaciones Fate, ases/bennies/cartas de
Savage Worlds, defensas GURPS, pools y sistemas sin dados.

### Autoridad

| Actor | Responsabilidad |
| --- | --- |
| LLM | Interpretar ficción, proponer intención, decidir PNJ solo si se delega y narrar el outcome |
| Ruleset | Legalidad, modificadores, costes, ventanas, outcomes y eventos propuestos |
| Host | Autenticar, generar azar, validar, proyectar, confirmar y auditar |
| DM | Resolver ambigüedades, dificultades/excepciones y elecciones asignadas al DM |
| Jugador | Declarar intención, escoger opciones y autorizar gastos propios |

Cada request pendiente indica autoridad y, si procede, participante concreto.
Delegar una decisión al LLM es configuración explícita y auditable. El LLM no
calcula la mecánica final ni edita JSON; narra solo datos confirmados. Una
excepción del DM también se registra.

### Eventos y RNG

El host mantiene snapshot materializado y event log mecánico. Este log es
distinto de `SessionState.Log`, que sigue siendo una vista narrativa. Cada evento
incluye secuencia, revisión, ruleset id/version/digest, resolución, request,
principal, tipo/schema, payload, timestamp del host y referencias de azar.

Todo azar procede del host: enteros uniformes, barajado/cartas y helpers de
dado. El paquete no accede a RNG, reloj o entropía propios. Cada draw se vincula
a `(request_id, resolution_id, draw_index)` y registra parámetros, fuente y
resultado; un retry reutiliza el draw. El replay conserva resultados efectivos
sin publicar una semilla predecible. Dados físicos entran como input externo y
quedan auditados como tales.

### Manifiesto, digest y lock

Cada bundle contiene `ruleset.json`. El manifiesto no declara su propio digest:
el instalador obtiene el manifiesto del bundle y calcula SHA-256 mientras lee
los bytes exactos e inmutables, produciendo un `Artifact` atestiguado por el
host. Así se evita un hash autorreferencial o elegido por el paquete.

El formato objetivo del manifiesto incluye:

- `id`, versión SemVer y versión del protocolo;
- rango compatible del engine, runtime y entrypoint;
- schemas de estado, eventos y acciones;
- capabilities, dependencias y locales;
- guía operativa del LLM y migraciones.

El primer esqueleto implementa identidad, nombre, versión, protocolo, runtime,
entrypoint y capabilities. Los demás campos se añadirán de forma aditiva junto
con el loader; no alteran el lock ni la frontera de resolución.

Para un bundle externo, el instalador/host calcula SHA-256 sobre sus bytes ZIP
exactos e inmutables. Para una release Go incorporada, calcula el digest sobre
un framing canónico de las fuentes ejecutables declaradas por el paquete y de
sus helpers compartidos explícitos (`ruleskit`, `diceexpr`, `jsonstrict`, según
el paquete). El código del kernel anfitrión no forma parte de ese artefacto:
`protocol_version` identifica por separado el contrato host-paquete. Un refactor
compatible del host no invalida locks; un cambio incompatible de envelopes,
validación o semántica de ejecución exige incrementar `ProtocolVersion`.

La sesión fija:

```text
id + version + digest + protocol_version
```

La aventura declara un requisito/rango; al crear sesión se resuelve un lock
exacto. Nunca hay upgrades silenciosos. Si falta el artefacto, la sesión solo
puede inspeccionarse/exportarse hasta instalarlo. Cambiarlo exige migración
explícita, validada y registrada.

Las releases incorporadas son inmutables. `internal/rules/runtimecatalog/
builtins.lock.json` es su ledger central y append-only: una entrada publicada no
se edita ni se elimina, y su implementación anterior se conserva registrable.
Cambiar una mecánica o un helper incluido requiere una versión SemVer nueva,
una definición nueva y añadir su lock al final del ledger; no se reescribe la
implementación antigua. El arranque valida que cada definición coincide con el
ledger y que cada entrada histórica tiene una implementación cargable. Cuando
cambie el protocolo, la compatibilidad o migración de locks anteriores debe
resolverse explícitamente antes de retirar soporte.

Rulesets y aventuras usan almacenes e instalaciones separados. El actual
`PackageModule` continúa limitado a `adventure.json` y `assets/`; importar una
aventura nunca instala ni ejecuta scripts.

### Runtime

1. **Go incorporado** para adaptar D&D 5e y estabilizar el contrato sin cambiar
   comportamiento.
2. **Starlark** para el MVP externo: builtins mínimos, sin SO/red/reloj/RNG,
   imports confinados, contexto cancelable, presupuesto de pasos y límites de
   entrada/salida/colecciones. Cada llamada usa un thread limpio.
3. **WASM** posteriormente para paquetes multilenguaje y aislamiento de memoria,
   manteniendo el mismo protocolo y tools.
4. **Lua** opcional para paquetes curados; no es el MVP porque endurecer cuotas
   de instrucciones/memoria y librerías requiere más trabajo.

Starlark no constituye por sí solo un sandbox completo. Si no puede garantizarse
una cuota de memoria en proceso, los paquetes no confiables se ejecutarán en un
worker aislado o esperarán a WASM.

### Seguridad

- capabilities denegadas por defecto;
- sin red, filesystem, procesos, entorno, reloj o RNG salvo servicio concedido;
- imports solo dentro del bundle, rechazando rutas absolutas, traversal,
  symlinks y colisiones;
- límites de bundle, tiempo, pasos, memoria/proceso, profundidad y eventos;
- schemas validados en ambas fronteras y tipos de evento permitidos;
- snapshots inmutables y commit atómico;
- proyecciones por principal para no filtrar secretos, HP o notas de DM;
- secretos de proveedor/configuración nunca llegan al paquete;
- auditoría de instalación, digest, permisos, migraciones, excepciones y azar.

La guía LLM también requiere capability y confianza. No puede concederse tools,
elevar autoridad ni anular la disciplina del host. Aventura, nombres, estado y
texto generado permanecen delimitados como datos no confiables.

## Migración específica del repositorio

1. Crear `internal/rules` con tipos/interfaces neutrales y tests del protocolo.
2. Registrar un adaptador Go `dnd5e` que delegue inicialmente en
   `internal/engine/tools.go`, conservando resultados.
3. Añadir requisito estructurado a `Adventure` y lock, estado opaco, eventos y
   pendientes a `SessionState` con campos `omitempty`. `system: "D&D 5e"` y
   sesiones sin lock migran al incorporado; un sistema desconocido no se adivina.
4. Componer `WorldTools`, `SessionTools` y `RulesTools`; implementar `game_*`
   sobre `types.Tool` y `mcptools.ToolProvider` para compartir API directa/MCP.
5. Añadir revisiones, idempotencia, event log y RNG host; integrar locking,
   autosave y journal sin convertir el log humano en event store.
6. Separar prompt neutral, modo y disciplina de información de la guía 5e;
   eliminar la suposición D&D del recap de `Oracle`.
7. Extraer gradualmente `Character`, `StatBlock`, `Feature.Skill/DC`, chargen,
   descansos, conjuros, XP e `internal/srd` detrás de 5e. El dominio común usa
   actor mínimo más `system_data`. El combate de
   `docs/tactical-combat-design.md` pertenece a 5e, no al kernel.
8. Añadir instalador/almacén Starlark separado del importador de aventuras.
9. Crear fixtures de conformidad para d20, d100, pool, PbtA, 3d6, Fate, Savage
   Worlds y un sistema sin dados; probar suspensión, replay, proyección y retries.
10. Añadir WASM/contenido modular solo tras estabilizar el protocolo con dos
    rulesets semánticamente distintos.

Cada fase deja cargables las sesiones existentes. No se retira una tool legacy
hasta migrar sus consumidores y mantener un periodo de compatibilidad.

### Estado de implementación (2026-08-18)

El corte vertical cubre ya el protocolo, la distribución local y la ejecución
durable de extremo a extremo:

- `internal/rules` implementa el contrato neutral, JSON estricto y acotado,
  unión `Step`, artefactos atestiguados por SHA-256, locks exactos y catálogo
  concurrente. El digest de cada paquete Go incorporado cubre sus fuentes
  ejecutables y helpers compartidos declarados, mientras `ProtocolVersion`
  versiona el contrato del kernel. El ledger append-only impide sobrescribir
  releases y obliga a conservar sus implementaciones;
- las sesiones persisten lock, estado inicial y materializado, revisión,
  generación, eventos, draws, continuaciones y recibos. El host hace CAS,
  checkpoint atómico antes de `Resume`, replay desde revisión cero y retries sin
  repetir azar ni `Reduce`, también después de reiniciar;
- las siete tools `game_*` resuelven exclusivamente el paquete fijado que el
  catálogo de la sesión inyectó. Las utilidades históricas 5e sólo se anuncian y
  ejecutan para el artefacto `dnd5e` incorporado exacto; no existe fallback desde
  un lock ajeno, ausente o inválido;
- hay diez paquetes Go de referencia: d20 (D&D 5e y Pathfinder 2e), d100
  (RuneQuest y Call of Cthulhu 7e), pools (Vampire 5e y Shadowrun 6e), PbtA,
  GURPS 4e, Fate Core y Savage Worlds. Cubren azar secuencial, grados, críticos,
  dados explosivos y una decisión humana persistible;
- el runtime Starlark carga bundles ZIP inmutables sin APIs de SO, red, reloj o
  entropía, con imports confinados, cuotas, cancelación, límites estructurales y
  caché por digest. El CLI permite empaquetar, validar, instalar, descubrir y
  listar releases sin mezclar código con aventuras;
- escritorio, servicio HTTP, bot, Oracle/MCP y editor cargan o declaran el mismo
  requisito. Abrir una sesión resuelve una versión compatible sólo una vez;
  reabrir exige el digest exacto y verifica replay antes de exponer mecánicas.

Quedan fuera deliberadamente las resoluciones hijas (`StartChild`, rechazado de
forma cerrada), el aislamiento de memoria mediante proceso/WASM, firmas de
editores, un marketplace y migraciones automáticas. Instalar un bundle requiere
reiniciar los procesos largos para reconstruir su catálogo inmutable y nunca
actualiza silenciosamente una campaña existente.

## Consecuencias

Ganamos reglas autoritativas, tools estables, trazabilidad, reintentos seguros,
aislamiento entre contenido y código y evolución de runtimes. El coste es añadir
registro, locks, schemas, continuaciones, proyecciones, eventos, RNG y migraciones.

Se descartan: un gran schema universal basado en HP/CA/iniciativa; paquetes por
familia de dado; una tool por acción; dejar la mecánica al LLM con `roll_dice`;
Lua o WASM como primer runtime; plugins nativos Go; scripts dentro de aventuras;
y actualizar automáticamente campañas al último ruleset.

## Criterios de aceptación

- `game_*` ejecuta al menos dos modelos de resolución diferentes.
- una resolución pendiente sobrevive a guardar/reabrir y respeta su autoridad;
- retries no duplican eventos ni draws;
- igual versión con digest diferente es rechazada;
- importar una aventura no instala código;
- proyecciones DM/jugador no filtran estado;
- módulos y sesiones 5e existentes migran sin cambio observable;
- replay reconstruye el mismo snapshot;
- timeout, panic o payload inválido no muta estado.

## Trabajo posterior

Marketplace, firmas, certificación, compactación y licencias se decidirán por
separado. No modifican la frontera, las tools ni el lock por digest adoptados.
