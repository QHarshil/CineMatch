/**
 * Minimal stand-in for the Supabase query builder. Every chained call returns
 * the same builder, and awaiting it resolves to the given rows, which is all
 * the pages need to render.
 */
export function supabaseStub(rows: unknown[] = []) {
  const result = { data: rows, error: null };
  const builder: object = new Proxy(
    {},
    {
      get(_target, prop) {
        if (prop === "then") {
          return (resolve: (value: typeof result) => unknown) =>
            Promise.resolve(result).then(resolve);
        }
        return () => builder;
      },
    },
  );
  return {
    from: () => builder,
    rpc: () => builder,
    auth: {
      getSession: async () => ({ data: { session: null } }),
      onAuthStateChange: () => ({ data: { subscription: { unsubscribe() {} } } }),
    },
  };
}
