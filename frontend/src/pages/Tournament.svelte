<script lang="ts">
/** @import * from '../../bindings/gui' */
import { onMount } from "svelte";
import toast from "svelte-5-french-toast";
import { App } from "../../bindings/gui/index.js";
import Landing from "../components/Tournament/Landing.svelte";
import Create from "../components/Tournament/Create.svelte";
import Bracket from "../components/Tournament/Bracket.svelte";
import SeedingEditor from "../components/Tournament/SeedingEditor.svelte";
import TeamRoster from "../components/Tournament/TeamRoster.svelte";
import AutoStartToolbar from "../components/Tournament/AutoStartToolbar.svelte";
import StatsPanel from "../components/Tournament/StatsPanel.svelte";
import HumanInfoModal from "../components/Tournament/HumanInfoModal.svelte";

// Tournament page. Orchestrates three views: landing (saved list + templates),
// create (the reworked create form), and bracket (the live tournament). The Go
// side is the source of truth; we re-fetch the whole state after every mutation.
// State objects use snake_case JSON tags; input payloads use camelCase.

let view = $state<"landing" | "create" | "bracket">("landing");
let state: any = $state(null);

async function refresh() {
  try {
    const s = await App.TournamentGetState();
    if (s) {
      state = s;
      view = "bracket";
      // Re-fetch the derived stats/history too, so a reloaded tournament shows
      // them (they are persisted inside each match's player_stats, but the
      // aggregated reports are computed live and must be re-fetched on mount).
      await refreshDerived();
    }
  } catch (e) {
    console.warn("refresh failed", e);
  }
}

onMount(refresh);

async function createTournament(cfg: any) {
  try {
    state = await App.TournamentNew(cfg);
    view = "bracket";
    await refreshDerived();
    toast.success("Tournament created");
  } catch (e) {
    toast.error(`Failed to create tournament: ${e}`, { duration: 10000 });
  }
}

async function openFromLanding(s: any) {
  state = s;
  view = "bracket";
  // The aggregated stats/history reports are computed live (not stored on the
  // loaded state), so they must be re-fetched whenever a tournament is opened.
  await refreshDerived();
}

async function deleteTournament() {
  try {
    await App.TournamentDelete();
    state = null;
    view = "landing";
    toast.success("Tournament deleted");
  } catch (e) {
    toast.error(`Failed to delete: ${e}`);
  }
}

async function saveToLibrary() {
  try {
    await App.TournamentSaveToLibrary();
    toast.success("Saved to library");
  } catch (e) {
    toast.error(`Failed to save: ${e}`);
  }
}

async function saveAsTemplate() {
  const name = prompt("Template name:", state?.name || "");
  if (name === null) return;
  try {
    await App.TournamentSaveCurrentAsTemplate(name);
    toast.success("Saved as template");
  } catch (e) {
    toast.error(`Failed to save template: ${e}`);
  }
}

async function exportJSON() {
  try {
    const json = await App.TournamentExportJSON();
    const blob = new Blob([json], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${state?.name || "tournament"}.json`;
    a.click();
    URL.revokeObjectURL(url);
  } catch (e) {
    toast.error(`Failed to export: ${e}`);
  }
}

function importJSON() {
  const input = document.createElement("input");
  input.type = "file";
  input.accept = ".json,application/json";
  input.onchange = async () => {
    const file = input.files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      const s = await App.TournamentImportJSON(text);
      state = s;
      view = "bracket";
      await refreshDerived();
      toast.success("Tournament imported");
    } catch (e) {
      toast.error(`Failed to import: ${e}`);
    }
  };
  input.click();
}

// --- match play ---
let startingMatchId = $state("");
let selectedMatchId = $state("");

// --- auto-start state (ported from v4 tournament-vue.js) ---
let autoStartEnabled = $state(false);
let autoStartInterval = $state(30);
let autoStartHumanBehavior = $state("pause"); // 'continue' | 'pause' | 'skip'
let autoStartCountdown = $state<number | null>(null);
let autoStartTimer: ReturnType<typeof setInterval> | null = null;

// --- stats / history / modal ---
let stats = $state<any>(null);
let history = $state<any>(null);
let playerStats = $state<any[]>([]);
let showHumanInfo = $state(false);

function matchHasHumans(m: any): boolean {
  for (const p of [m.participant1, m.participant2]) {
    if (p && p.participant_type === "human") return true;
  }
  for (const t of [m.team1, m.team2]) {
    if (t?.participants?.some((p: any) => p.participant_type === "human")) return true;
  }
  return false;
}

function groupByRound(matches: any[]) {
  const byRound: Record<number, any[]> = {};
  for (const m of matches || []) (byRound[m.round_num] ||= []).push(m);
  return Object.keys(byRound)
    .map(Number)
    .sort((a, b) => a - b)
    .map((rn) => ({ roundNum: rn, matches: byRound[rn] }));
}

// Find the next match that can be auto-started. For double elimination, prefer
// LB matches that are ready, only playing WB matches when LB is waiting on WB
// losers. When ignoreHumanBehavior (manual start), the human behavior setting is
// ignored so 'pause'/'skip' never block a manual start.
function findNextAutoStartMatch(ignoreHumanBehavior = false): any {
  const s = state;
  if (!s) return null;
  const isDouble = s.format === "double_elimination";

  const wbAll = isDouble
    ? (s.matches || []).filter((m: any) => String(m.match_id).startsWith("W"))
    : s.matches || [];
  const lbAll = isDouble
    ? [
        ...(s.losers_bracket_matches || []),
        ...(s.matches || []).filter((m: any) => String(m.match_id).startsWith("L")),
      ]
    : [];

  const wbRounds = groupByRound(wbAll);
  const lbRounds = groupByRound(lbAll);
  const order = isDouble ? [...lbRounds, ...wbRounds] : [...wbRounds, ...lbRounds];

  for (const rd of order) {
    for (const m of rd.matches) {
      if (m.completed) continue;
      if (!m.participant1 || !m.participant2) continue;
      if (!ignoreHumanBehavior && matchHasHumans(m)) {
        if (autoStartHumanBehavior === "skip") continue;
        if (autoStartHumanBehavior === "pause") return null;
        // 'continue' → proceed with this match
      }
      return m;
    }
  }
  return null;
}

function clearAutoStartTimer() {
  if (autoStartTimer) {
    clearInterval(autoStartTimer);
    autoStartTimer = null;
  }
  autoStartCountdown = null;
}

function startAutoStartTimer() {
  clearAutoStartTimer();
  const next = findNextAutoStartMatch(false);
  if (!next) {
    autoStartEnabled = false;
    return;
  }
  if (matchHasHumans(next) && autoStartHumanBehavior === "pause") {
    // Wait for the operator to start the human match manually.
    return;
  }
  if (autoStartInterval === 0) {
    launchMatch(next);
    return;
  }
  autoStartCountdown = autoStartInterval;
  autoStartTimer = setInterval(() => {
    if (autoStartCountdown === null) return;
    autoStartCountdown--;
    if (autoStartCountdown <= 0) {
      clearAutoStartTimer();
      launchMatch(next);
    }
  }, 1000);
}

function toggleAutoStart() {
  if (autoStartEnabled) {
    autoStartEnabled = false;
    clearAutoStartTimer();
  } else {
    autoStartEnabled = true;
    startAutoStartTimer();
  }
}

// Start the selected match (manual) or the next available match (auto).
async function startSelectedMatch(manual = false) {
  if (selectedMatchId) {
    const m = findMatchById(selectedMatchId);
    selectedMatchId = "";
    if (m) await launchMatch(m);
    return;
  }
  const next = findNextAutoStartMatch(manual);
  if (next) await launchMatch(next);
}

function findMatchById(id: string): any {
  if (!state) return null;
  const all = [...(state.matches || []), ...(state.losers_bracket_matches || [])];
  return all.find((m: any) => m.match_id === id) || null;
}

async function launchMatch(match: any) {
  if (!match || match.completed) return;
  if (!match.participant1 || !match.participant2) return;
  selectedMatchId = "";
  startingMatchId = match.match_id;
  try {
    const s = await App.TournamentStartMatch(match.match_id, false);
    state = s;
    await refreshDerived();
    // After a match completes, schedule the next one if auto-start is on.
    if (autoStartEnabled && !state?.completed) {
      setTimeout(() => startAutoStartTimer(), 1000);
    }
  } catch (e) {
    toast.error(`Match failed: ${e}`, { duration: 10000 });
  } finally {
    startingMatchId = "";
  }
}

// Abort the running match (v4 parity: stop_match). Clears the in-progress state
// and auto-start timer so the operator can fix the issue and re-start the match.
async function stopMatch() {
  clearAutoStartTimer();
  autoStartEnabled = false;
  try {
    await App.TournamentStopMatch();
    // svelte-5-french-toast has no .info(); the default export is itself callable
    // for a neutral (blank-type) toast.
    toast("Match stopped");
  } catch (e) {
    toast.error(`Failed to stop match: ${e}`);
  } finally {
    startingMatchId = "";
  }
}

async function manualRecord(match: any, winnerName: string) {
  const s0 = match.score?.[0] ?? 0;
  const s1 = match.score?.[1] ?? 0;
  try {
    state = await App.TournamentRecordResult(match.match_id, winnerName, [s0, s1], [], 0, false);
    await refreshDerived();
  } catch (e) {
    toast.error(`Record failed: ${e}`);
  }
}

async function openRocketLeague() {
  try {
    await App.TournamentOpenRocketLeague();
    toast.success("Rocket League launched into freeplay");
  } catch (e) {
    toast.error(`Failed to open Rocket League: ${e}`, { duration: 10000 });
  }
}

async function repairBracket() {
  try {
    const s = await App.TournamentRepairBracket();
    state = s;
    await refreshDerived();
    toast.success("Bracket repaired from feeder results");
  } catch (e) {
    toast.error(`Repair failed: ${e}`, { duration: 10000 });
  }
}

async function refreshDerived() {
  try {
    stats = await App.TournamentGetStats();
  } catch (e) {
    console.warn("stats failed", e);
  }
  try {
    history = await App.TournamentGetMatchHistory();
  } catch (e) {
    console.warn("history failed", e);
  }
  try {
    playerStats = await App.TournamentGetPlayerStats();
  } catch (e) {
    console.warn("player stats failed", e);
  }
}

let winnerName = $derived(
  state?.winner_team?.name || state?.winner?.name || "",
);
</script>

{#if view === "landing"}
  <Landing onCreate={() => (view = "create")} onOpen={openFromLanding} />
{:else if view === "create"}
  <Create onCreate={createTournament} onCancel={() => (view = "landing")} />
{:else}
  <div class="page">
    <div class="box blurred header">
      <button onclick={() => (view = "landing")}>← Back</button>
      <h2>{state.name}</h2>
      <span class="tag">{state.format}</span>
      <span class="tag">{state.team_size}v{state.team_size}</span>
      {#if winnerName}<span class="tag win">Winner: {winnerName}</span>{/if}
      <div style="flex:1"></div>
      <SeedingEditor tournament={state} onUpdate={(s) => (state = s)} />
      <button onclick={saveToLibrary}>Save</button>
      <button onclick={saveAsTemplate}>Save as Template</button>
      <button onclick={exportJSON}>Export</button>
      <button onclick={importJSON}>Import</button>
      {#if state.format === "double_elimination"}
        <button onclick={repairBracket} title="Re-derive bracket participants from feeder results (fixes save/load corruption)">🔧 Repair Bracket</button>
      {/if}
      <button onclick={deleteTournament}>Delete</button>
    </div>

    {#if !state.completed}
      <AutoStartToolbar
        bind:interval={autoStartInterval}
        bind:humanBehavior={autoStartHumanBehavior}
        enabled={autoStartEnabled}
        countdown={autoStartCountdown}
        matchInProgress={startingMatchId !== ""}
        selectedMatchId={selectedMatchId}
        onStart={(manual) => startSelectedMatch(manual)}
        onStop={stopMatch}
        onOpenRL={openRocketLeague}
        onShowInfo={() => (showHumanInfo = true)}
        onToggle={toggleAutoStart}
      />
    {/if}

    <TeamRoster tournament={state} onUpdate={(s) => (state = s)} />

    <Bracket
      tournament={state}
      {startingMatchId}
      bind:selectedMatchId
      onPlay={(m) => launchMatch(m)}
      onRecord={manualRecord}
    />

    <StatsPanel {stats} {history} {playerStats} />
  </div>
{/if}

<HumanInfoModal bind:visible={showHumanInfo} tournament={state} onOpenRL={openRocketLeague} />

<style>
  .page {
    padding: 1rem;
    height: 100%;
    width: 100%;
    overflow: auto;
  }
  .box {
    border-radius: 0.4rem;
    background-color: var(--background);
    padding: 0.6rem;
  }
  .header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
    margin-bottom: 1rem;
  }
  .tag {
    background: var(--foreground);
    color: var(--background);
    border-radius: 0.3rem;
    padding: 0.1rem 0.4rem;
    font-size: 0.75rem;
  }
  .tag.win {
    background: gold;
    color: #000;
  }
</style>
