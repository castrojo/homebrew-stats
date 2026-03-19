/**
 * Safe JSON serialisation for use with Astro's set:html directive.
 *
 * JSON.stringify does NOT escape `<`, so values containing `</script>` would
 * break HTML parsing when injected via set:html into a <script> element.
 * Unicode-escaping `<` as `\u003c` is valid JSON (parsers decode it) and is
 * invisible to the browser HTML parser.
 */
export function safeJson(data: unknown): string {
  return JSON.stringify(data).replace(/</g, '\\u003c');
}

/**
 * Reads and parses JSON data injected into a <script type="application/json"> element.
 *
 * Use page.evaluate() in Playwright tests — NOT page.locator() — to read these elements,
 * as Playwright's locator API does not reliably find <script> elements.
 *
 * Throws with a descriptive message if the element is missing or the JSON is invalid.
 */
export function readChartData<T>(id: string): T {
  const el = document.getElementById(id);
  if (!el) throw new Error(`[readChartData] Element #${id} not found in DOM`);
  try {
    return JSON.parse(el.textContent ?? '') as T;
  } catch (e) {
    throw new Error(`[readChartData] Failed to parse JSON from #${id}: ${String(e)}`);
  }
}
