<script lang="ts">
/** @import * from '../../../bindings/gui' */
import { onMount } from "svelte";
import toast from "svelte-5-french-toast";
import { App, type BotInfo } from "../../../bindings/gui/index.js";
import { mutators as mutatorOptions } from "../MatchSettings/rlmutators";
import MutatorSettings from "./MutatorSettings.svelte";
import LauncherSelector from "../LauncherSelector.svelte";
import Modal from "../Modal.svelte";
import { parseJSON } from "../../index";

// The tournament create form, ported from the v4 fork's modals.html create
// tournament modal. Unlike the old minimal v5 form, bots are selected from a
// pool of scanned bot cards (no forced name entry) and humans are added via a
// count selector with optional custom names. Map/game-mode/mutators come from
// the shared MutatorSettings component (human-readable map names + presets).

let {
  onCreate,
  onCancel,
}: {
  onCreate: (cfg: any) => void;
  onCancel: () => void;
} = $props();

let name = $state(localStorage.getItem("TOUR_NAME") || "My Tournament");
let format = $state(localStorage.getItem("TOUR_FORMAT") || "single_elimination");
let teamSize = $state(Number(localStorage.getItem("TOUR_TEAMSIZE") || "1"));
let allowDuplicates = $state(localStorage.getItem("TOUR_DUP") === "true");
let swissRounds = $state(Number(localStorage.getItem("TOUR_SWISS") || "0"));
let mercyRule = $state(Number(localStorage.getItem("TOUR_MERCY") || "0"));

let map = $state(localStorage.getItem("TOUR_MAP") || "Stadium_P");
let mode = $state(localStorage.getItem("TOUR_GAMEMODE") || "Soccar");
let randomizeMap = $state(localStorage.getItem("TOUR_RANDOMIZE") === "true");
let mutators: { [k: string]: number } = $state(
  parseJSON(localStorage.getItem("TOUR_MUTATORS")) ||
    Object.fromEntries(Object.keys(mutatorOptions).map((k) => [k, 0])),
);

$effect(() => localStorage.setItem("TOUR_NAME", name));
$effect(() => localStorage.setItem("TOUR_FORMAT", format));
$effect(() => localStorage.setItem("TOUR_TEAMSIZE", String(teamSize)));
$effect(() => localStorage.setItem("TOUR_DUP", String(allowDuplicates)));
$effect(() => localStorage.setItem("TOUR_SWISS", String(swissRounds)));
$effect(() => localStorage.setItem("TOUR_MAP", map));
$effect(() => localStorage.setItem("TOUR_GAMEMODE", mode));
$effect(() => localStorage.setItem("TOUR_RANDOMIZE", String(randomizeMap)));
$effect(() => localStorage.setItem("TOUR_MUTATORS", JSON.stringify(mutators)));
$effect(() => localStorage.setItem("TOUR_MERCY", String(mercyRule)));

// --- bot pool (scanned from the same search paths as the home screen) ---
let bots: BotInfo[] = $state([]);
let scripts: BotInfo[] = $state([]);
let loading = $state(false);
let searchQuery = $state("");
let selectedBotPaths = $state<string[]>(
  parseJSON(localStorage.getItem("TOUR_SELECTED_BOTS")) || [],
);
let enabledScriptPaths = $state<string[]>(
  parseJSON(localStorage.getItem("TOUR_ENABLED_SCRIPTS")) || [],
);

$effect(() =>
  localStorage.setItem("TOUR_SELECTED_BOTS", JSON.stringify(selectedBotPaths)),
);
$effect(() =>
  localStorage.setItem("TOUR_ENABLED_SCRIPTS", JSON.stringify(enabledScriptPaths)),
);

let humanCount = $state(Number(localStorage.getItem("TOUR_HUMANS") || "0"));
let humanNames: string[] = $state(
  parseJSON(localStorage.getItem("TOUR_HUMAN_NAMES")) || [],
);
$effect(() => localStorage.setItem("TOUR_HUMANS", String(humanCount)));
$effect(() =>
  localStorage.setItem("TOUR_HUMAN_NAMES", JSON.stringify(humanNames)),
);

let showLauncher = $state(false);

function visiblePaths(): string[] {
  const paths = parseJSON(localStorage.getItem("BOT_SEARCH_PATHS")) || [];
  return paths.filter((p: any) => p.visible).map((p: any) => p.installPath);
}

async function rescan() {
  loading = true;
  try {
    const paths = visiblePaths();
    const [b, s] = await Promise.all([App.GetBots(paths), App.GetScripts(paths)]);
    bots = b;
    scripts = s;
  } catch (e) {
    toast.error(`Failed to scan bots/scripts: ${e}`);
  } finally {
    loading = false;
  }
}

onMount(rescan);

const filteredBots = $derived(
  searchQuery
    ? bots.filter((b) =>
        b.config.settings.name.toLowerCase().includes(searchQuery.toLowerCase()),
      )
    : bots,
);

function toggleBot(path: string) {
  if (selectedBotPaths.includes(path)) {
    selectedBotPaths = selectedBotPaths.filter((p) => p !== path);
  } else {
    selectedBotPaths = [...selectedBotPaths, path];
  }
}
function toggleScript(path: string) {
  if (enabledScriptPaths.includes(path)) {
    enabledScriptPaths = enabledScriptPaths.filter((p) => p !== path);
  } else {
    enabledScriptPaths = [...enabledScriptPaths, path];
  }
}
function setHumanCount(n: number) {
  humanCount = n;
  const next = humanNames.slice(0, n);
  while (next.length < n) next.push("");
  humanNames = next;
}

function submit() {
  const participants: any[] = [];
  for (const path of selectedBotPaths) {
    const bot = bots.find((b) => b.tomlPath === path);
    if (!bot) continue;
    participants.push({
      name: bot.config.settings.name,
      participantType: "bot",
      // Strip the base64 icon so saved tournament files stay small.
      player: {
        sort: "rlbot",
        player: { config: bot.config, tomlPath: bot.tomlPath, loadout: bot.loadout, icon: "" },
      },
    });
  }
  for (let i = 0; i < humanCount; i++) {
    participants.push({
      name: humanNames[i]?.trim() || `Human ${i + 1}`,
      participantType: "human",
      player: { sort: "human", player: {} },
    });
  }
  if (participants.length === 0) {
    toast.error("Select at least one bot or add a human player");
    return;
  }

  const cfg = {
    name,
    format,
    participants,
    teamSize,
    allowDuplicates,
    swissRounds,
    map,
    gameMode: mode,
    randomizeMap,
    scripts: scripts.filter((s) => enabledScriptPaths.includes(s.tomlPath)),
    mercyRule,
    mutators,
    extraOptions:
      parseJSON(localStorage.getItem("MS_EXTRAOPTIONS")) || {},
    launcher: localStorage.getItem("MS_LAUNCHER") || "nolaunch",
    launcherArg: localStorage.getItem("MS_LAUNCHER_ARG") || "",
  };
  onCreate(cfg);
}
</script>

<div class="page">
  <div class="col">
    <div class="box blurred">
      <h2>Create Tournament</h2>
      <label>Name <input type="text" bind:value={name} /></label>
      <label>
        Format
        <select bind:value={format}>
          <option value="single_elimination">Single Elimination</option>
          <option value="double_elimination">Double Elimination</option>
          <option value="round_robin">Round Robin</option>
          <option value="swiss">Swiss</option>
        </select>
      </label>
      <label>
        Team Size
        <select bind:value={teamSize}>
          {#each [1, 2, 3, 4, 5] as n}<option value={n}>{n}v{n}</option>{/each}
        </select>
      </label>
      {#if format === "swiss"}
        <label>
          Swiss Rounds (0 = auto)
          <input type="number" bind:value={swissRounds} min="0" />
        </label>
      {/if}
      <label class="check">
        <input type="checkbox" bind:checked={allowDuplicates} /> Allow Duplicates (party mode)
      </label>
      <label>
        Mercy Rule (0 = off)
        <input type="number" bind:value={mercyRule} min="0" />
      </label>
      <button onclick={() => (showLauncher = true)}>Launcher Options</button>
    </div>

    <div class="box blurred">
      <h3>Match Settings</h3>
      <MutatorSettings
        bind:map
        bind:mode={mode}
        bind:mutators
        bind:randomizeMap
      />
    </div>

    {#if scripts.length > 0}
      <div class="box blurred">
        <h3>Scripts</h3>
        <div class="list">
          {#each scripts as s (s.tomlPath)}
            <label class="check">
              <input
                type="checkbox"
                checked={enabledScriptPaths.includes(s.tomlPath)}
                onchange={() => toggleScript(s.tomlPath)}
              />
              {s.config.settings.name}
            </label>
          {/each}
        </div>
      </div>
    {/if}
  </div>

  <div class="col">
    <div class="box blurred">
      <h3>
        Bots ({selectedBotPaths.length} selected)
        {#if loading}<span class="muted">searching…</span>{/if}
      </h3>
      <input type="search" placeholder="Search bots…" bind:value={searchQuery} />
      <div class="cards">
        {#each filteredBots as b (b.tomlPath)}
          <button
            class="card"
            class:selected={selectedBotPaths.includes(b.tomlPath)}
            onclick={() => toggleBot(b.tomlPath)}
          >
            {#if b.icon}<img src={b.icon} alt="" />{/if}
            <span>{b.config.settings.name}</span>
          </button>
        {/each}
      </div>
    </div>

    <div class="box blurred">
      <h3>Human Players</h3>
      <label>
        Count
        <input
          type="number"
          min="0"
          max="10"
          value={humanCount}
          onchange={(e) => setHumanCount(Number((e.target as HTMLInputElement).value))}
        />
      </label>
      {#each Array(humanCount) as _, i}
        <input
          type="text"
          placeholder="Player name"
          bind:value={humanNames[i]}
        />
      {/each}
    </div>

    <div class="actions">
      <button class="primary" onclick={submit}>Create Tournament</button>
      <button onclick={onCancel}>Cancel</button>
    </div>
  </div>
</div>

<Modal title="Select a launcher" bind:visible={showLauncher}>
  <LauncherSelector />
</Modal>

<style>
  .page {
    padding: 1rem;
    height: 100%;
    width: 100%;
    display: flex;
    gap: 1rem;
    overflow: auto;
    align-items: flex-start;
  }
  .col {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    min-width: 0;
  }
  .box {
    border-radius: 0.4rem;
    background-color: var(--background);
    padding: 0.6rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .box label {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    font-size: 0.85rem;
  }
  .box label.check {
    flex-direction: row;
    align-items: center;
    gap: 0.4rem;
  }
  .muted {
    font-weight: normal;
    opacity: 0.6;
    font-size: 0.8rem;
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }
  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
    gap: 0.4rem;
    max-height: 50vh;
    overflow-y: auto;
  }
  .card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.2rem;
    padding: 0.4rem;
    border-radius: 0.4rem;
    border: 2px solid transparent;
    background: var(--background-alt, #2a2a2a);
    cursor: pointer;
    font-size: 0.75rem;
  }
  .card img {
    width: 40px;
    height: 40px;
    object-fit: contain;
  }
  .card.selected {
    border-color: var(--orange, #ff8c00);
  }
  .actions {
    display: flex;
    gap: 0.5rem;
  }
  .actions button {
    flex: 1;
  }
</style>
