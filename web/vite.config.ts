import path from "path";
import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const proxyTarget = env.VITE_API_PROXY_TARGET || "http://127.0.0.1:8080";

  const vendorChunkGroups: Array<{ name: string; packages: string[] }> = [
    {
      name: "react-vendor",
      packages: ["react", "react-dom", "scheduler"],
    },
    {
      name: "router-vendor",
      packages: ["react-router", "react-router-dom", "@remix-run/router"],
    },
    {
      name: "i18n-vendor",
      packages: ["i18next", "react-i18next"],
    },
    {
      name: "graph-vendor",
      packages: ["@xyflow/react"],
    },
    {
      name: "render-vendor",
      packages: ["react-syntax-highlighter", "diff2html"],
    },
    {
      name: "state-vendor",
      packages: ["zustand"],
    },
  ];

  const readPackageName = (id: string): string | null => {
    const normalized = id.replace(/\\/g, "/");
    const marker = "/node_modules/";
    const markerIndex = normalized.lastIndexOf(marker);
    if (markerIndex < 0) {
      return null;
    }
    const packagePath = normalized.slice(markerIndex + marker.length);
    if (!packagePath) {
      return null;
    }
    if (packagePath.startsWith("@")) {
      const [scope, name] = packagePath.split("/", 3);
      return scope && name ? `${scope}/${name}` : null;
    }
    const [name] = packagePath.split("/", 2);
    return name || null;
  };

  return {
    plugins: [react()],
    resolve: {
      alias: {
        "@": path.resolve(__dirname, "./src"),
      },
    },
    build: {
      crossOriginLoading: false,
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (!id.includes("node_modules")) {
              return undefined;
            }
            const packageName = readPackageName(id);
            if (!packageName) {
              return undefined;
            }
            for (const group of vendorChunkGroups) {
              if (group.packages.includes(packageName)) {
                return group.name;
              }
            }
            return "vendor";
          },
        },
      },
    },
    server: {
      proxy: {
        "/api": {
          target: proxyTarget,
          changeOrigin: true,
          ws: true
        }
      }
    }
  };
});
