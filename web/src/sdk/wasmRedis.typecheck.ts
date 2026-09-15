import type { WasmRedis } from "./wasmRedis";

type ExampleSchema = {
  name: string;
  age: number;
};

// Ce fichier est compile par TypeScript : il prouve le typage sans être exécuté.
export function checkTypedQueries(db: WasmRedis<ExampleSchema>): void {
  db.get().where("age", ">", 18);
  db.get().where("name", "contains", "ma");

  // @ts-expect-error Un âge attend un nombre.
  db.get().where("age", ">", "18");
  // @ts-expect-error "contains" n'est pas autorisé pour un nombre.
  db.get().where("age", "contains", 18);
  // @ts-expect-error Ce champ n'existe pas dans le schéma.
  db.get().where("unknown", "equals", "value");
}
