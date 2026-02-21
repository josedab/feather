/**
 * Feather TypeScript Transform SDK
 *
 * Write feature transformations in TypeScript that compile to WASM
 * and deploy to Feather's runtime.
 */

/** Interface describing a feature transform. */
export interface FeatherTransform {
  name: string;
  inputSchema: Record<string, string>;
  outputSchema: Record<string, string>;
  transform: (input: Record<string, any>) => Record<string, any>;
}

/** Internal registry of transforms. */
const registry: Map<string, FeatherTransform> = new Map();

/**
 * Register a transform in the global registry.
 *
 * @param t - The transform definition to register.
 * @throws Error if a transform with the same name is already registered.
 */
export function registerTransform(t: FeatherTransform): void {
  if (registry.has(t.name)) {
    throw new Error(`Transform '${t.name}' is already registered`);
  }
  registry.set(t.name, t);
}

/**
 * List all registered transforms (without exposing the transform function).
 *
 * @returns Array of transform metadata objects.
 */
export function listTransforms(): Array<{
  name: string;
  inputSchema: Record<string, string>;
  outputSchema: Record<string, string>;
}> {
  return Array.from(registry.values()).map(({ name, inputSchema, outputSchema }) => ({
    name,
    inputSchema,
    outputSchema,
  }));
}

/**
 * Run a registered transform locally with test data.
 *
 * @param name - Name of the registered transform.
 * @param input - Input data matching the transform's input schema.
 * @returns The transform's output.
 * @throws Error if the transform is not found.
 */
export function testTransform(
  name: string,
  input: Record<string, any>,
): Record<string, any> {
  const t = registry.get(name);
  if (!t) {
    throw new Error(`Transform '${name}' is not registered`);
  }
  return t.transform(input);
}

// ---------------------------------------------------------------------------
// Built-in example transform
// ---------------------------------------------------------------------------

registerTransform({
  name: "normalizeScore",
  inputSchema: { raw_score: "float", min_score: "float", max_score: "float" },
  outputSchema: { normalized_score: "float" },
  transform: (input: Record<string, any>): Record<string, any> => {
    const raw = input.raw_score as number;
    const min = input.min_score as number;
    const max = input.max_score as number;
    if (max === min) {
      return { normalized_score: 0.0 };
    }
    return { normalized_score: (raw - min) / (max - min) };
  },
});
