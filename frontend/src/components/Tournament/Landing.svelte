<script lang="ts">
/** @import * from '../../../bindings/gui' */
import { onMount } from "svelte";
import toast from "svelte-5-french-toast";
import { App } from "../../../bindings/gui/index.js";

// Tournament landing page, ported from the v4 fork's landing.html. Shows the
// saved-tournament library and reusable templates, plus create/import actions.
// The active tournament (if any) is shown at the top with a resume button.

let {
  onCreate,
  onOpen,
}: {
  onCreate: () => void;
  onOpen: (state: any) => void;
} = $props();

let saved: any[] = $state([]);
let templates: any[] = $state([]);
let loading = $state(false);

async function refresh() {
  loading = true;
  try {
    const [s, t] = await Promise.all([
      App.TournamentListSaved(),
      App.TournamentListTemplates(),
    ]);
    saved = s || [];
    templates = t || [];
  } catch (e) {
    console.warn("refresh failed", e);
  } finally {
    loading = false;
  }
}

onMount(refresh);

async function open(id: string) {
  try {
    const state = await App.TournamentLoadFromID(id);
    onOpen(state);
  } catch (e) {
    toast.error(`Failed to load: ${e}`);
  }
}

async function remove(id: string) {
  try {
    await App.TournamentDeleteFromList(id);
    await refresh();
  } catch (e) {
    toast.error(`Failed to delete: ${e}`);
  }
}

async function removeTemplate(id: string) {
  try {
    await App.TournamentDeleteTemplate(id);
    await refresh();
  } catch (e) {
    toast.error(`Failed to delete template: ${e}`);
  }
}

// Reads a .json file the user picks and returns its text, or null if cancelled.
function pickJSON(): Promise<string | null> {
  return new Promise((resolve) => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = ".json,application/json";
    input.onchange = async () => {
      const file = input.files?.[0];
      if (!file) return resolve(null);
      try {
        resolve(await file.text());
      } catch {
        resolve(null);
      }
    };
    input.click();
  });
}

async function importTournament() {
  const text = await pickJSON();
  if (text == null) return;
  try {
    const state = await App.TournamentImportJSON(text);
    onOpen(state);
    toast.success("Tournament imported");
  } catch (e) {
    toast.error(`Failed to import tournament: ${e}`, { duration: 10000 });
  }
}

async function importTemplate() {
  const text = await pickJSON();
  if (text == null) return;
  try {
    await App.TournamentImportTemplateJSON(text);
    await refresh();
    toast.success("Template imported");
  } catch (e) {
    toast.error(`Failed to import template: ${e}`, { duration: 10000 });
  }
}

async function exportTemplate(t: any) {
  try {
    const json = await App.TournamentExportTemplateJSON(t.template_id);
    const blob = new Blob([json], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${t.name || "template"}.json`;
    a.click();
    URL.revokeObjectURL(url);
  } catch (e) {
    toast.error(`Failed to export template: ${e}`);
  }
}

async function useTemplate(t: any) {
  // Load the template config into the create form by writing it to localStorage
  // and opening the create view. The create form reads these keys on mount.
  const c = t.config || {};
  localStorage.setItem("TOUR_NAME", c.name || "My Tournament");
  localStorage.setItem("TOUR_FORMAT", c.format || "single_elimination");
  localStorage.setItem("TOUR_TEAMSIZE", String(c.teamSize ?? 1));
  localStorage.setItem("TOUR_DUP", String(!!c.allowDuplicates));
  localStorage.setItem("TOUR_SWISS", String(c.swissRounds ?? 0));
  localStorage.setItem("TOUR_MAP", c.map || "Stadium_P");
  localStorage.setItem("TOUR_GAMEMODE", c.gameMode || "Soccar");
  localStorage.setItem("TOUR_RANDOMIZE", String(!!c.randomizeMap));
  localStorage.setItem("TOUR_MUTATORS", JSON.stringify(c.mutators || {}));
  localStorage.setItem("TOUR_MERCY", String(c.mercyRule ?? 0));
  localStorage.setItem("TOUR_SELECTED_BOTS", JSON.stringify(
    (c.participants || []).filter((p: any) => p.participantType === "bot")
      .map((p: any) => p.player?.player?.tomlPath).filter(Boolean),
  ));
  const humans = (c.participants || []).filter((p: any) => p.participantType === "human");
  localStorage.setItem("TOUR_HUMANS", String(humans.length));
  localStorage.setItem("TOUR_HUMAN_NAMES", JSON.stringify(humans.map((p: any) => p.name)));
  localStorage.setItem("TOUR_ENABLED_SCRIPTS", JSON.stringify(
    (c.scripts || []).map((s: any) => s.tomlPath).filter(Boolean),
  ));
  onCreate();
}

function formatMapName(mapId: string): string {
  if (!mapId) return "";
  return mapId
    .replace(/_/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2");
}

function formatDate(iso: string): string {
  if (!iso) return "";
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

function formatFormat(f: string): string {
  return f
    .split("_")
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(" ");
}
</script>

<div class="page">
  <div class="header">
    <h1>Tournaments</h1>
    <div style="flex:1"></div>
    <button onclick={importTemplate}>Import Template</button>
    <button onclick={importTournament}>Import Tournament</button>
    <button class="primary" onclick={onCreate}>+ Create New Tournament</button>
  </div>

  <div class="columns">
    <div class="box blurred">
      <h2>Saved Tournaments</h2>
      {#if loading}<p class="muted">Loading…</p>{/if}
      {#if !loading && saved.length === 0}<p class="muted">No saved tournaments.</p>{/if}
      <div class="list">
        {#each saved as s (s.tournament_id)}
          <div class="item">
            <div class="info">
              <strong>{s.name}</strong>
              <span class="tag">{formatFormat(s.format)}</span>
              <span class="tag">{s.team_size}v{s.team_size}</span>
              <span class="tag">{s.participant_count} participants</span>
              {#if s.completed}<span class="tag win">Winner: {s.winner_name}</span>{/if}
              <span class="muted small">{formatDate(s.created_at)}</span>
            </div>
            <div class="itemActions">
              <button onclick={() => open(s.tournament_id)}>Open</button>
              <button class="danger" onclick={() => remove(s.tournament_id)}>Delete</button>
            </div>
          </div>
        {/each}
      </div>
    </div>

    <div class="box blurred">
      <h2>Templates</h2>
      {#if !loading && templates.length === 0}<p class="muted">No templates saved.</p>{/if}
      <div class="list">
        {#each templates as t (t.template_id)}
          <div class="item">
            <div class="info">
              <strong>{t.name}</strong>
              <span class="tag">{formatFormat(t.config?.format || "")}</span>
              <span class="tag">{formatMapName(t.config?.map || "")}</span>
              <span class="muted small">{formatDate(t.created_at)}</span>
            </div>
            <div class="itemActions">
              <button onclick={() => useTemplate(t)}>Use</button>
              <button onclick={() => exportTemplate(t)}>Export</button>
              <button class="danger" onclick={() => removeTemplate(t.template_id)}>Delete</button>
            </div>
          </div>
        {/each}
      </div>
    </div>
  </div>
</div>

<style>
  .page {
    padding: 1rem;
    height: 100%;
    width: 100%;
    overflow: auto;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }
  .header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .columns {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
    align-items: start;
  }
  @media (max-width: 800px) {
    .columns {
      grid-template-columns: 1fr;
    }
  }
  .box {
    border-radius: 0.4rem;
    background-color: var(--background);
    padding: 0.6rem;
  }
  .list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    margin-top: 0.5rem;
  }
  .item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
    padding: 0.4rem;
    border-radius: 0.3rem;
    background: var(--background-alt, #2a2a2a);
    flex-wrap: wrap;
  }
  .info {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    min-width: 0;
  }
  .itemActions {
    display: flex;
    gap: 0.3rem;
  }
  .tag {
    background: var(--foreground);
    color: var(--background);
    border-radius: 0.3rem;
    padding: 0.05rem 0.35rem;
    font-size: 0.7rem;
    align-self: flex-start;
  }
  .tag.win {
    background: gold;
    color: #000;
  }
  .muted {
    opacity: 0.7;
  }
  .small {
    font-size: 0.7rem;
  }
  button.danger {
    background: #cc1414;
    color: white;
  }
</style>
