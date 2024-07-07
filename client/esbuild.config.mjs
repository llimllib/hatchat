import { build } from "esbuild";

build({
  entryPoints: ["src/index.ts"],
  outfile: "../static/client.js",
  bundle: true,
  minify: true,
  sourcemap: true,
  target: ["esnext"],
  format: "esm",
  plugins: [],
}).catch(() => process.exit(1));
