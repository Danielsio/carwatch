import { describe, it, expect } from "vitest";
import { Trie, buildTrie } from "./trie";

interface Make {
  id: number;
  name: string;
  name_he?: string;
}

const makes: Make[] = [
  { id: 1, name: "Toyota", name_he: "טויוטה" },
  { id: 2, name: "Land Rover", name_he: "לנד רובר" },
  { id: 3, name: "Mercedes-Benz", name_he: "מרצדס" },
  { id: 4, name: "Mazda", name_he: "מאזדה" },
];

function build() {
  return buildTrie(makes, (m) => [m.name, m.name_he]);
}

describe("Trie", () => {
  it("returns all values for an empty or whitespace query", () => {
    const trie = build();
    expect(trie.search("").map((m) => m.id)).toEqual([1, 2, 3, 4]);
    expect(trie.search("   ").map((m) => m.id)).toEqual([1, 2, 3, 4]);
  });

  it("matches an English prefix case-insensitively", () => {
    const trie = build();
    expect(trie.search("toy").map((m) => m.id)).toEqual([1]);
    expect(trie.search("TOY").map((m) => m.id)).toEqual([1]);
  });

  it("matches a Hebrew prefix", () => {
    const trie = build();
    expect(trie.search("מרצ").map((m) => m.id)).toEqual([3]);
  });

  it("matches a prefix of any word, not just the first", () => {
    const trie = build();
    // "rov" is the second word of "Land Rover"
    expect(trie.search("rov").map((m) => m.id)).toEqual([2]);
  });

  it("splits on punctuation so 'benz' matches 'Mercedes-Benz'", () => {
    const trie = build();
    expect(trie.search("benz").map((m) => m.id)).toEqual([3]);
  });

  it("AND-intersects multi-word queries", () => {
    const trie = build();
    expect(trie.search("land rov").map((m) => m.id)).toEqual([2]);
    // both prefixes must match the same item
    expect(trie.search("land toy")).toEqual([]);
  });

  it("returns multiple matches in insertion order", () => {
    const trie = build();
    // "ma" prefixes Mercedes? no. Mazda yes. Add ambiguity:
    expect(trie.search("ma").map((m) => m.id)).toEqual([4]);
  });

  it("returns an empty array when nothing matches", () => {
    const trie = build();
    expect(trie.search("xyz")).toEqual([]);
  });

  it("ignores undefined keywords via buildTrie filtering", () => {
    const trie = buildTrie(
      [{ id: 9, name: "Kia", name_he: undefined }],
      (m: Make) => [m.name, m.name_he],
    );
    expect(trie.search("ki").map((m) => m.id)).toEqual([9]);
    expect(trie.search("").map((m) => m.id)).toEqual([9]);
  });

  it("dedupes a value indexed under overlapping keywords", () => {
    const trie = new Trie<Make>();
    trie.insert({ id: 1, name: "Audi" }, ["Audi", "Audi", "audi"]);
    expect(trie.search("au").map((m) => m.id)).toEqual([1]);
  });
});
