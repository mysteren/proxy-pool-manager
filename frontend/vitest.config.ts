import { fileURLToPath, URL } from "node:url";

import { defineConfig } from "vitest/config";

// Отдельный конфиг тестов: не тянет Wails/Vite-плагины из vite.config.ts, поэтому тесты
// быстрые и не зависят от сборки. Алиасы синхронизированы с tsconfig.json / vite.config.ts.
export default defineConfig({
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
