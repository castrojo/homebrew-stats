import tseslint from "@typescript-eslint/eslint-plugin";
import tsParser from "@typescript-eslint/parser";
import astroPlugin from "eslint-plugin-astro";

export default [
  // TypeScript files
  {
    files: ["src/**/*.ts"],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        project: "./tsconfig.json",
      },
    },
    plugins: {
      "@typescript-eslint": tseslint,
    },
    rules: {
      ...tseslint.configs.recommended.rules,
      // Allow intentionally-unused params/vars when prefixed with _
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
    },
  },

  // Astro files — use eslint-plugin-astro recommended config
  ...astroPlugin.configs.recommended,

  // Project-wide rules applying to Astro template sections
  {
    files: ["**/*.astro"],
    rules: {
      // CRITICAL: set:text HTML-encodes quotes. Browsers do NOT decode HTML entities
      // inside <script> raw text elements. Use set:html for <script type="application/json">.
      // See: skills/bootc-ecosystem/SKILL.md#chart-data-injection
      "no-restricted-syntax": [
        "error",
        {
          selector: "JSXAttribute[name.name='set:text']",
          message:
            "Use set:html (not set:text) for <script type=\"application/json\"> elements. " +
            "set:text HTML-encodes quotes which JSON.parse cannot read inside script elements.",
        },
      ],
    },
  },

  {
    ignores: ["dist/", "node_modules/"],
  },
];
