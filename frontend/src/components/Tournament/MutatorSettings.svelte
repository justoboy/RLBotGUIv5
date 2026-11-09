<script lang="ts">
import { fly } from "svelte/transition";
import { MAPS_NON_STANDARD, MAPS_STANDARD } from "../../arena-names";
import NiceSelect from "../NiceSelect.svelte";
import Select from "../NiceSelect.svelte";
import Modal from "../Modal.svelte";
import { type Gamemode, gamemodes } from "../MatchSettings/rlmodes";
import { mutatorCategories, mutators as mutatorOptions } from "../MatchSettings/rlmutators";

// A self-contained, reusable version of the map / game-mode / mutator picker from
// components/MatchSettings/Main.svelte, minus the Start/Stop buttons and the
// launcher selector (the tournament reads the launcher from localStorage). Used
// by the tournament create form so it gets the same full mutator grid + presets
// and human-readable map names as the normal match screen.

let {
  map = $bindable(),
  mode = $bindable(),
  mutators = $bindable(),
  randomizeMap = $bindable(),
}: {
  map: string;
  mode: string;
  mutators: { [k: string]: number };
  randomizeMap: boolean;
} = $props();

let showMutators = $state(false);
let mutatorSearchQuery = $state("");
$effect(() => {
  if (!showMutators) {
    mutatorSearchQuery = "";
  }
});

const RANDOM_STANDARD_MAP = "__random_standard_map__";

let mapOptionSelected = $state<string>(randomizeMap ? RANDOM_STANDARD_MAP : map);

// Keep `map` concrete (never the random sentinel) so callers always get a real
// map, and propagate the randomize flag to the parent.
$effect(() => {
  if (mapOptionSelected === RANDOM_STANDARD_MAP) {
    randomizeMap = true;
  } else {
    randomizeMap = false;
    if (mapOptionSelected !== map) {
      map = mapOptionSelected;
    }
  }
});

function cleanCase(toClean: string): string {
  const halfClean = toClean.replaceAll("_", " ");
  return halfClean.charAt(0).toUpperCase() + halfClean.slice(1);
}

function buildMutatorCategories(): { name: string; keys: string[] }[] {
  const allMutatorKeys = Object.keys(mutators);
  const seen = new Set<string>();
  const categories: { name: string; keys: string[] }[] = [];

  for (const [name, keys] of Object.entries(mutatorCategories)) {
    for (const key of keys) {
      if (seen.has(key)) {
        console.warn(`Mutator "${key}" appears in multiple categories`);
      }
      seen.add(key);
    }
    categories.push({ name, keys: [...keys] });
  }

  const uncategorized = allMutatorKeys.filter(
    (key) => key !== "game_mode" && !seen.has(key),
  );
  if (uncategorized.length > 0) {
    categories.push({ name: "other", keys: uncategorized });
  }

  for (const cat of categories) {
    cat.keys.sort();
  }
  return categories;
}

const mutatorCategoryOptions = buildMutatorCategories();
const allMutatorKeys = mutatorCategoryOptions.flatMap((c) => c.keys);

function filterMutatorCategories(
  categories: { name: string; keys: string[] }[],
  query: string,
): { name: string; keys: string[] }[] {
  if (!query) return categories;
  const lowerQuery = query.toLowerCase();
  return categories
    .map(({ name, keys }) => ({
      name,
      keys: keys.filter((key) => cleanCase(key).toLowerCase().includes(lowerQuery)),
    }))
    .filter((group) => group.keys.length > 0);
}

let searchedMutatorOptions = $derived(
  filterMutatorCategories(mutatorCategoryOptions, mutatorSearchQuery),
);

function resetMutators() {
  for (const key of Object.keys(mutators)) {
    mutators[key] = 0;
  }
  selectedPreset = "";
}

let selectedPreset: Gamemode | "" = $state.raw("");
$effect(() => {
  if (selectedPreset !== "") {
    setPreset(selectedPreset);
  }
});

function setPreset(presetData: Gamemode) {
  if (presetData.match.game_mode !== undefined) {
    mode = presetData.match.game_mode;
  }
  mapOptionSelected = presetData.match.game_map_upk ?? RANDOM_STANDARD_MAP;
  for (const key of allMutatorKeys) {
    if (presetData.mutators[key] !== undefined) {
      mutators[key] = mutatorOptions[key].indexOf(presetData.mutators[key]);
    } else {
      mutators[key] = 0;
    }
  }
}

function countModifiedMutators(): number {
  let count = 0;
  for (const key of Object.keys(mutators)) {
    if (mutators[key] !== 0) count++;
  }
  return count;
}
let numModifiedMutators = $derived(countModifiedMutators());

function getMaps(): { [k: string]: string } {
  const standardMaps = Object.entries(MAPS_STANDARD).sort(([a], [b]) =>
    a.localeCompare(b),
  );
  const nonStandardMaps = Object.entries(MAPS_NON_STANDARD).sort(([a], [b]) =>
    a.localeCompare(b),
  );
  return Object.fromEntries([...standardMaps, ...nonStandardMaps]);
}

const ALL_MAPS = getMaps();
const MAP_OPTIONS = {
  "Random Standard Map": RANDOM_STANDARD_MAP,
  ...ALL_MAPS,
};
</script>

<div class="settings">
  <div class="left-controls">
    <Select
      options={MAP_OPTIONS}
      bind:value={mapOptionSelected}
      placeholder="Select map"
    />
    <Select
      options={Object.fromEntries(mutatorOptions.game_mode.map((x) => [x, x]))}
      bind:value={mode}
      placeholder="Select mode"
    />
  </div>
  <div class="right-controls">
    <button onclick={() => { showMutators = true; }}>
      Mutators
      {#if numModifiedMutators != 0}<span>{numModifiedMutators}</span>{/if}
    </button>
  </div>
</div>

{#if showMutators}
  <Modal title="Rocket League Mutators" bind:visible={showMutators}>
    {#snippet children()}
      <div class="mutator-search">
        <input
          type="search"
          placeholder="Search mutators…"
          bind:value={mutatorSearchQuery}
        />
      </div>
      <div class="mutators">
        {#each searchedMutatorOptions as { name, keys } (name)}
          <div class="category-header" in:fly={{ duration: 300, y: 8 }}>{cleanCase(name)}</div>
          {#each keys as mutatorKey (mutatorKey)}
            <div class="mutator" in:fly={{ duration: 300, y: 8 }}>
              <label
                class={mutators[mutatorKey] == 0 ? "" : "mutatorChanged"}
                for={mutatorKey}>{cleanCase(mutatorKey)}</label>
              <select
                name={mutatorKey}
                id={mutatorKey}
                bind:value={mutators[mutatorKey]}
                onchange={() => { selectedPreset = ""; }}
              >
                {#each mutatorOptions[mutatorKey] as value, i}
                  <option value={i}>{value}</option>
                {/each}
              </select>
            </div>
          {/each}
        {/each}
      </div>
    {/snippet}
    {#snippet footer()}
      <div class="bottomButtons">
        <p>Settings are saved automatically</p>
        <NiceSelect bind:value={selectedPreset} options={gamemodes} placeholder="Select a preset" />
        <button class="mutatorResetButton" onclick={resetMutators}>Reset</button>
      </div>
    {/snippet}
  </Modal>
{/if}

<style>
  .settings {
    display: flex;
    justify-content: space-between;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .left-controls,
  .right-controls {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .mutator-search input {
    width: 100%;
    box-sizing: border-box;
    padding: 0.4rem 0.6rem;
    border-radius: 0.3rem;
    border: 1px solid var(--border, #555);
    background: var(--background, #222);
    color: var(--foreground, #eee);
    font-size: 0.9rem;
  }
  .mutatorsBody {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }
  .mutators {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    display: grid;
    grid-template-columns: auto auto auto auto auto;
    gap: 0.4rem;
    align-content: start;
  }
  .category-header {
    grid-column: 1 / -1;
    font-weight: bold;
    margin-top: 0.4rem;
  }
  .mutator {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .mutator label {
    font-size: 0.75rem;
  }
  .mutatorChanged {
    color: var(--orange, #ff8c00);
  }
  .mutator select {
    padding: 0.2rem;
  }
  .bottomButtons {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .bottomButtons p {
    flex: 1;
    font-size: 0.8rem;
    opacity: 0.8;
  }
</style>
