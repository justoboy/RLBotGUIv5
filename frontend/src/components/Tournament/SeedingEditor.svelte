<script lang="ts">
/** @import * from '../../../bindings/gui' */
import { App } from "../../../bindings/gui/index.js";
import toast from "svelte-5-french-toast";
import Modal from "../Modal.svelte";

/**
 * Seeding + team-pairing editor, ported from the v4 fork's seeding-editor and
 * manual-pairing modals (tournament-vue.js). Lets the user reorder seeds by
 * click-to-swap, shuffle the order, rename teams, and (in team mode) force
 * specific participants onto the same team before auto-forming the rest.
 *
 * The Go side is the source of truth: every mutation returns the fresh state,
 * which we hand back via onUpdate so the bracket re-renders.
 */
let {
  tournament,
  onUpdate,
}: {
  tournament: any;
  onUpdate: (state: any) => void;
} = $props();

let visible = $state(false);
let selectedSwap = $state(-1); // index into participants for click-to-swap
let selectedPairing: string[] = $state([]); // participant_ids selected for pairing
let manualPairings: [string, string][] = $state([]);

let participants = $derived(tournament?.participants || []);
let teamSize = $derived(tournament?.team_size || 1);
let isTeamMode = $derived(teamSize > 1);
let teams = $derived(tournament?.teams || []);

function open() {
  selectedSwap = -1;
  selectedPairing = [];
  manualPairings = [];
  visible = true;
}

function isPaired(pid: string) {
  return manualPairings.some(([a, b]) => a === pid || b === pid);
}

function nameOf(pid: string) {
  const p = participants.find((x: any) => x.participant_id === pid);
  return p?.name || pid;
}

async function handleClick(index: number) {
  if (selectedSwap === -1) {
    selectedSwap = index;
  } else if (selectedSwap === index) {
    selectedSwap = -1;
  } else {
    await doSwap(selectedSwap, index);
    selectedSwap = -1;
  }
}

async function doSwap(i1: number, i2: number) {
  const p1 = participants[i1];
  const p2 = participants[i2];
  if (!p1 || !p2) return;
  try {
    const state = await App.TournamentSwapSeeds(p1.participant_id, p2.participant_id);
    onUpdate(state);
  } catch (e) {
    toast.error(`Failed to swap seeds: ${e}`);
  }
}

async function randomize() {
  try {
    const state = await App.TournamentRandomizeSeeding();
    onUpdate(state);
  } catch (e) {
    toast.error(`Failed to randomize: ${e}`);
  }
}

function togglePairing(pid: string) {
  if (selectedPairing.includes(pid)) {
    selectedPairing = selectedPairing.filter((x) => x !== pid);
  } else {
    selectedPairing = [...selectedPairing, pid];
  }
}

function addPairing() {
  if (selectedPairing.length !== 2) return;
  const [a, b] = selectedPairing;
  // Append the edge; do NOT wipe existing pairings that share a member. The Go
  // side groups participants into teams by connected components, so chaining
  // edges (A-B, then B-C) grows a single team to 3+ members. The old code wiped
  // any edge sharing a member, which made it impossible to exceed 2 per team.
  if (!manualPairings.some(([x, y]) => (x === a && y === b) || (x === b && y === a))) {
    manualPairings = [...manualPairings, [a, b]];
  }
  selectedPairing = [];
}

function removePairing(idx: number) {
  manualPairings = manualPairings.filter((_, i) => i !== idx);
}

function clearPairings() {
  manualPairings = [];
  selectedPairing = [];
}

async function applyPairings() {
  try {
    const state = await App.TournamentFormTeamsWithPairings(manualPairings, []);
    onUpdate(state);
    toast.success("Teams formed");
  } catch (e) {
    toast.error(`Failed to form teams: ${e}`);
  }
}

async function renameTeam(index: number, name: string) {
  try {
    const state = await App.TournamentRenameTeam(index, name);
    onUpdate(state);
  } catch (e) {
    toast.error(`Failed to rename team: ${e}`);
  }
}
</script>

<button onclick={open}>Edit Seeding</button>

<Modal title="Seeding & Teams" bind:visible>
  <div class="editor">
    <p class="hint">
      Click a participant to select, then click another to swap their positions.
      Use Randomize to shuffle the order.
    </p>

    <div class="row">
      <button onclick={randomize}>⤨ Randomize</button>
    </div>

    <div class="seeding-list">
      {#each participants as p, i (p.participant_id)}
        <div
          class="seeding-item"
          class:selected={selectedSwap === i}
          role="button"
          tabindex={0}
          onclick={() => handleClick(i)}
          onkeydown={(e) => e.key === "Enter" && handleClick(i)}
        >
          <span class="rank">#{i + 1}</span>
          <span class="type">{p.participant_type === "human" ? "👤" : "🤖"}</span>
          <span class="name">{p.name}</span>
          {#if selectedSwap === i}<span class="action">(click another to swap)</span>{/if}
        </div>
      {/each}
    </div>

    {#if isTeamMode}
      <hr />
      <h3>Manual Team Pairing</h3>
      <p class="hint">
        Select two participants and click "Pair Together" to force them onto the
        same team, then "Form Teams". Team size: <strong>{teamSize}</strong>
      </p>
      <div class="pairing-grid">
        <div class="pairing-list">
          {#each participants as p (p.participant_id)}
            <div
              class="pairing-item"
              class:selected={selectedPairing.includes(p.participant_id)}
              class:paired={isPaired(p.participant_id)}
              role="button"
              tabindex={0}
              onclick={() => togglePairing(p.participant_id)}
              onkeydown={(e) => e.key === "Enter" && togglePairing(p.participant_id)}
            >
              <span>{p.participant_type === "human" ? "👤" : "🤖"}</span>
              <span class="name">{p.name}</span>
              {#if isPaired(p.participant_id)}<span class="badge">✓</span>{/if}
            </div>
          {/each}
        </div>
        <div class="pairing-side">
          <div class="row">
            <button disabled={selectedPairing.length !== 2} onclick={addPairing}>
              Pair Together
            </button>
            <button onclick={clearPairings}>Clear Pairings</button>
          </div>
          {#if manualPairings.length > 0}
            <div class="current-pairings">
              <h4>Current Pairings</h4>
              {#each manualPairings as pair, idx (idx)}
                <div class="pairing-current">
                  <span>{nameOf(pair[0])} & {nameOf(pair[1])}</span>
                  <button onclick={() => removePairing(idx)}>✕</button>
                </div>
              {/each}
            </div>
          {/if}
          <button class="primary" onclick={applyPairings}>Form Teams</button>
        </div>
      </div>

      {#if teams.length > 0}
        <hr />
        <h3>Team Names</h3>
        <div class="team-names">
          {#each teams as t, i (t.team_id)}
            <label class="team-name-row">
              <span>Team {i + 1}</span>
              <input
                type="text"
                value={t.name}
                onchange={(e) => renameTeam(i, e.currentTarget.value)}
              />
            </label>
          {/each}
        </div>
      {/if}
    {/if}
  </div>
</Modal>

<style>
  .editor {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    min-width: 320px;
    max-width: 70vw;
  }
  .hint {
    font-size: 0.78rem;
    opacity: 0.75;
    margin: 0;
  }
  .row {
    display: flex;
    gap: 0.4rem;
  }
  .seeding-list {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    max-height: 40vh;
    overflow-y: auto;
  }
  .seeding-item {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.35rem 0.5rem;
    border-radius: 0.3rem;
    background: var(--background-alt, #2a2a2a);
    cursor: pointer;
  }
  .seeding-item.selected {
    outline: 2px solid #00d9ff;
  }
  .seeding-item .rank {
    font-weight: 700;
    opacity: 0.7;
    min-width: 1.6rem;
  }
  .seeding-item .name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .seeding-item .action {
    font-size: 0.7rem;
    opacity: 0.6;
  }
  .pairing-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.6rem;
    align-items: start;
  }
  .pairing-list {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    max-height: 40vh;
    overflow-y: auto;
  }
  .pairing-item {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.3rem 0.45rem;
    border-radius: 0.3rem;
    background: var(--background-alt, #2a2a2a);
    cursor: pointer;
  }
  .pairing-item.selected {
    outline: 2px solid #00d9ff;
  }
  .pairing-item.paired {
    opacity: 0.55;
  }
  .pairing-item .name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .pairing-item .badge {
    color: #28a745;
  }
  .pairing-side {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .current-pairings {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .pairing-current {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.4rem;
    padding: 0.25rem 0.4rem;
    border-radius: 0.3rem;
    background: var(--background-alt, #2a2a2a);
    font-size: 0.8rem;
  }
  .team-names {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }
  .team-name-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.8rem;
  }
  .team-name-row span {
    min-width: 3.5rem;
    opacity: 0.7;
  }
  .team-name-row input {
    flex: 1;
  }
  hr {
    width: 100%;
    border: none;
    border-top: 1px solid var(--background-alt, #333);
    margin: 0.2rem 0;
  }
  h3 {
    margin: 0;
    font-size: 0.9rem;
  }
  h4 {
    margin: 0;
    font-size: 0.8rem;
  }
</style>
