/**
 * Word-level prefix trie for fast typeahead search over a small in-memory
 * catalog (manufacturers / models). Each value is indexed under every
 * whitespace/punctuation-separated token of its searchable strings, so typing a
 * prefix of *any* word matches it — e.g. "rov" matches "Land Rover", and the
 * Hebrew "מרצ" matches "מרצדס". Multi-word queries are AND-ed across tokens
 * ("land rov" → "Land Rover"). Search is case-insensitive and returns matches in
 * insertion order so the caller's original ordering is preserved.
 *
 * Why a trie rather than `Array.filter(includes)`: prefix lookups are O(prefix
 * length) regardless of catalog size, and every node caches the set of value
 * ids beneath it, so collecting matches for a prefix is O(matches) with no
 * per-keystroke scan of the full list.
 */

interface TrieNode {
  children: Map<string, TrieNode>;
  /** Value ids whose token passes through this node (i.e. has this prefix). */
  ids: Set<number>;
}

function createNode(): TrieNode {
  return { children: new Map(), ids: new Set() };
}

function normalize(text: string): string {
  return text.toLowerCase().trim();
}

/** Split a string into searchable tokens on whitespace and common separators. */
function tokenize(text: string): string[] {
  return normalize(text)
    .split(/[\s/\\\-_(),.]+/)
    .filter(Boolean);
}

function intersect(a: Set<number>, b: Set<number>): Set<number> {
  const [small, large] = a.size <= b.size ? [a, b] : [b, a];
  const out = new Set<number>();
  for (const id of small) {
    if (large.has(id)) out.add(id);
  }
  return out;
}

export class Trie<T> {
  private readonly root = createNode();
  private readonly values: T[] = [];

  /** Index `value` under each token of every provided keyword. */
  insert(value: T, keywords: string[]): void {
    const id = this.values.length;
    this.values.push(value);
    const seen = new Set<string>();
    for (const keyword of keywords) {
      for (const token of tokenize(keyword)) {
        if (seen.has(token)) continue;
        seen.add(token);
        this.insertToken(token, id);
      }
    }
  }

  private insertToken(token: string, id: number): void {
    let node = this.root;
    node.ids.add(id);
    for (const char of token) {
      let next = node.children.get(char);
      if (!next) {
        next = createNode();
        node.children.set(char, next);
      }
      node = next;
      node.ids.add(id);
    }
  }

  /**
   * Return values that have a token starting with each token in `query`
   * (AND semantics), in insertion order. An empty/whitespace query returns all
   * values.
   */
  search(query: string): T[] {
    const tokens = tokenize(query);
    if (tokens.length === 0) return [...this.values];

    let matched: Set<number> | null = null;
    for (const token of tokens) {
      const ids = this.collect(token);
      matched = matched === null ? ids : intersect(matched, ids);
      if (matched.size === 0) return [];
    }

    return [...(matched ?? [])]
      .sort((a, b) => a - b)
      .map((id) => this.values[id]);
  }

  /** Ids of all values whose any token starts with `prefix`. */
  private collect(prefix: string): Set<number> {
    let node = this.root;
    for (const char of prefix) {
      const next = node.children.get(char);
      if (!next) return new Set();
      node = next;
    }
    return node.ids;
  }
}

/** Build a trie from a list, deriving each item's searchable keywords. */
export function buildTrie<T>(
  items: T[],
  getKeywords: (item: T) => Array<string | undefined>,
): Trie<T> {
  const trie = new Trie<T>();
  for (const item of items) {
    const keywords = getKeywords(item).filter(
      (k): k is string => Boolean(k),
    );
    trie.insert(item, keywords);
  }
  return trie;
}
