# Bundles de reglas Starlark

El runtime `internal/rules/starlarkruntime` ejecuta paquetes de reglas externos
sin darles acceso a servicios del proceso anfitrión. Un paquete es un ZIP
inmutable que contiene:

- `ruleset.json`, un `rules.Manifest` estricto y sin campos desconocidos.
- El `runtime.entrypoint` indicado por el manifiesto, con extensión `.star`.
- Cero o más módulos `.star` importables desde la raíz del bundle.
- Recursos adicionales opcionales. Forman parte del digest, aunque el runtime
  no ofrece una API para leerlos.

## Carga e instalación

```go
loader, err := starlarkruntime.NewLoader(starlarkruntime.Limits{})
if err != nil {
    return err
}

loaded, err := loader.Load(ctx, bundleReader)
if err != nil {
    return err
}

// loaded.Artifact identifica exactamente los bytes ZIP consumidos.
// loaded.Ruleset implementa rules.Ruleset.
// loaded.InitialState es el JSON que debe recibir una sesión nueva.
err = catalog.Register(ctx, loaded.Artifact, loaded.Ruleset, loaded.InitialState)
```

`Loader.Load` calcula el SHA-256 sobre todos los bytes ZIP recibidos y construye
el artefacto mediante `rules.NewArtifact(manifest, bytes.NewReader(rawBundle))`.
No vuelve a empaquetar ni normaliza el archivo. Dos ZIP semánticamente iguales
pero byte a byte distintos son dos artefactos distintos. El publicador debe
conservar y distribuir exactamente el archivo sobre el que se creó el lock.

El loader conserva programas compilados y módulos congelados en una caché
FIFO, indexada por ese digest exacto. Volver a cargar los mismos bytes reutiliza
el mismo ruleset inmutable.

## Flujo de autoría

El repositorio incluye un empaquetador reproducible que aplica los mismos
límites y reglas de rutas que el loader:

```bash
make build-rules
./bin/thaimaturgy-rules pack ./mi-paquete ./mi-paquete.rules.zip
./bin/thaimaturgy-rules install ./mi-paquete.rules.zip
```

Los procesos de la aplicación cargan un catálogo inmutable al arrancar. Después
de instalar un bundle hay que reiniciar las instancias de escritorio, servidor
o bot que deban resolverlo para sesiones nuevas. Una sesión ya fijada conserva
siempre su lock exacto; reiniciar no la actualiza silenciosamente.

El directorio fuente debe contener `ruleset.json`, el entrypoint y cualquier
módulo o recurso adicional. No puede contener symlinks, dispositivos ni rutas
no portables, y la salida debe quedar fuera del árbol fuente. El ZIP se valida
y ejecuta desde un archivo temporal; una salida inválida nunca se publica.
Archivos fuente idénticos producen bytes y digest idénticos.

`examples/rules/simple-d6` muestra las diez funciones del contrato y un flujo
con estado: pide `dice.roll`, emite un evento, lo reduce de forma reproducible y
termina después de que el host confirma la revisión persistida.

## Manifiesto

Ejemplo de `ruleset.json`:

```json
{
  "id": "example.rules",
  "name": "Example Rules",
  "version": "1.0.0",
  "protocol_version": "1.0.0",
  "runtime": {
    "kind": "starlark",
    "entrypoint": "main.star"
  },
  "capabilities": ["example.echo"]
}
```

El entrypoint debe declarar `manifest()` y devolver exactamente los mismos
datos. Una diferencia produce `rules.ErrManifestMismatch`; por tanto, el código
ejecutable no puede suplantar la identidad atestiguada del bundle.

## Contrato del entrypoint

El módulo principal exporta estas funciones:

| Función | Argumento | Resultado |
| --- | --- | --- |
| `manifest()` | ninguno | objeto `rules.Manifest` |
| `initial_state()` | ninguno | valor JSON inicial, distinto de `null` |
| `list_actions(request)` | `rules.CatalogRequest` | lista de `rules.ActionDescriptor` |
| `start(request)` | `rules.StartRequest` | `rules.Step` |
| `resume(request)` | `rules.ResumeRequest` | `rules.Step` |
| `project(request)` | `rules.ProjectRequest` | `rules.Projection` |
| `explain(request)` | `rules.ExplainRequest` | `rules.Explanation` |
| `validate_state(request)` | `rules.ValidateStateRequest` | `None` o diagnóstico textual |
| `reduce(request)` | `rules.ReduceRequest` | `rules.ReduceResult` |
| `migrate(request)` | `rules.MigrateRequest` | `rules.MigrateResult` |

Los nombres de campos son los nombres JSON del protocolo Go. Las funciones
reciben estructuras nuevas y no comparten estado mutable entre invocaciones.
`validate_state` devuelve `None` cuando el snapshot es válido, o una cadena no
vacía cuando incumple una regla propia del paquete.

Todos los resultados vuelven a decodificarse de forma estricta y se validan con
el kernel. No se aceptan campos desconocidos, variantes `Step` incoherentes ni
un snapshot con un lock diferente al artefacto cargado.

## Datos neutrales

La frontera admite únicamente equivalentes JSON:

- `None`, booleanos, strings UTF-8, enteros y floats finitos.
- `list` para arrays.
- `dict` con claves string para objetos.

Tuplas, sets, bytes, funciones, claves no textuales, ciclos y valores no finitos
se rechazan al salir del script. Los objetos de entrada se ordenan por clave y
los de salida se serializan de forma estable, de modo que el orden textual de
los miembros de un objeto JSON no cambia el resultado mecánico. Los decimales
JSON se representan como floats IEEE-754; los enteros conservan precisión
arbitraria mientras respeten los límites de tamaño.

## Imports y sandbox

`load("rules/common.star", "helper")` resuelve siempre desde la raíz del mismo
ZIP. Se rechazan rutas absolutas, backslashes, segmentos vacíos, `.` o `..`,
caracteres no portables, módulos inexistentes, extensiones distintas de
`.star`, ciclos y entradas ZIP duplicadas o que sean symlinks/dispositivos.
Nunca se extrae el ZIP al filesystem.

El entorno no predeclara funciones o módulos de la aplicación. El script sólo
dispone del universo estándar determinista de Starlark. `print` se descarta y
no existe ninguna API de filesystem, procesos, red, reloj o entropía. El paquete
debe solicitar azar, decisiones, adjudicación, hijos y mutaciones mediante un
`rules.Step`, para que el host pueda auditarlos.

Se usa el dialecto conservador de Starlark: sin `while`, recursión, control de
flujo en el nivel superior ni reasignación global. Los módulos quedan
congelados tras inicializarse y pueden ejecutarse concurrentemente.

## Límites por defecto

| Recurso | Límite |
| --- | ---: |
| ZIP comprimido | 8 MiB |
| Total expandido | 8 MiB |
| Fuente `.star` individual | 1 MiB |
| Archivos | 64 |
| Pasos por inicialización o llamada | 100.000 |
| Profundidad de un valor | 32 |
| Nodos de un valor | 16.384 |
| Miembros por colección | 256 |
| Request o resultado completo | 4 MiB |
| Bundles compilados en caché | 64 |
| Cada `rules.Payload` | 1 MiB, impuesto por el kernel |

Los límites se pueden reducir mediante `Limits`. Un campo a cero conserva el
valor por defecto. No se pueden ampliar las colecciones por encima del máximo
del protocolo.

La cuota de pasos se aplica tanto a la inicialización de módulos como a cada
función. La cancelación o deadline del `context.Context` interrumpe activamente
el thread Starlark. El runtime es deliberadamente *in-process*: los límites
reducen el consumo accidental o abusivo, pero un despliegue que acepte autores
completamente hostiles debe añadir aislamiento de proceso y límites de memoria
del sistema operativo.

El host rechaza actualmente `StartChild`: las resoluciones que coordinan otro
ruleset quedan reservadas para una fase posterior. Tampoco existe todavía un
runtime WASM, firma de editores ni marketplace; la instalación local explícita
y el SHA-256 exacto son la frontera de distribución de este corte.

La implementación usa la API oficial de
[`go.starlark.net/starlark`](https://pkg.go.dev/go.starlark.net/starlark) y el
[lenguaje Starlark](https://github.com/google/starlark-go/blob/master/doc/spec.md).
