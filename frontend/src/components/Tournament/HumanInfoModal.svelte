<script lang="ts">
/**
 * Human Match Info modal (ported from v4's human-info-modal). Explains the
 * human LAN staging flow (Open RL → host/join via Rocket Plugin → Start Match)
 * and shows the current tournament settings (game mode, map, mercy rule, and
 * every mutator mapped from its enum index to a human label via rlmutators).
 */
import Modal from "../Modal.svelte";
import { mutatorCategories, mutators as mutatorOptions } from "../MatchSettings/rlmutators";
import { MAPS_STANDARD, MAPS_NON_STANDARD } from "../../arena-names";

let {
  visible = $bindable(false),
  tournament,
  onOpenRL,
}: {
  visible?: boolean;
  tournament: any;
  onOpenRL: () => void;
} = $props();

const ALL_MAPS: Record<string, string> = { ...MAPS_STANDARD, ...MAPS_NON_STANDARD };
// Reverse lookup: package id -> display name.
const ID_TO_DISPLAY: Record<string, string> = Object.fromEntries(
  Object.entries(ALL_MAPS).map(([display, id]) => [id, display]),
);

function mapDisplay(): string {
  const m = tournament?.map || "";
  if (!m) return "Default (DFH Stadium)";
  return ID_TO_DISPLAY[m] || m;
}

function mutatorLabel(key: string, idx: number): string {
  const list = mutatorOptions[key];
  if (!list || idx == null || idx < 0 || idx >= list.length) return "—";
  return list[idx];
}

function cleanCase(s: string): string {
  const h = s.replaceAll("_", " ");
  return h.charAt(0).toUpperCase() + h.slice(1);
}

// Ordered list of [key, label] pairs across all categories.
const mutatorKeys = Object.values(mutatorCategories).flat();

// Player count for the next available (or expected) match.
let counts = $derived.by(() => {
  const all = [...(tournament?.matches || []), ...(tournament?.losers_bracket_matches || [])];
  const match = all.find((m: any) => !m.completed && m.participant1 && m.participant2);
  const teamSize = tournament?.team_size || 1;
  if (match) {
    if (match.team1?.participants && match.team2?.participants) {
      return { t1: match.team1.participants.length, t2: match.team2.participants.length };
    }
    return { t1: 1, t2: 1 };
  }
  return { t1: teamSize, t2: teamSize };
});
</script>

<Modal title="Human Match Setup" bind:visible={visible}>
  {#snippet children()}
    <div class="body">
      <h5>Setting Up a Human Match</h5>
      <ol>
        <li>Click <b>Open Rocket League</b> to launch the game into a freeplay lobby.</li>
        <li>Host: press <b>Home</b> → <b>Host</b> in Rocket Plugin.</li>
        <li>Ensure <b>Continue and Spawn</b> is selected in RLBot GUI Settings.</li>
        <li>Human players: press <b>Home</b> → <b>Join</b> in Rocket Plugin.</li>
        <li>When all players are ready, click <b>Start Match</b>.</li>
      </ol>

      <h6>Current Tournament Settings</h6>
      <ul class="settings">
        <li><b>Game Mode:</b> {tournament?.game_mode || "Soccar"}</li>
        {#if tournament?.randomize_map}
          <li><b>Map:</b> Randomized per match</li>
        {:else}
          <li><b>Map:</b> {mapDisplay()}</li>
        {/if}
        <li><b>Mercy Rule:</b> {tournament?.mercy_rule ? `${tournament.mercy_rule} goal lead` : "None"}</li>
      </ul>

      <h6>Player Count</h6>
      <p>👥 Total players: {counts.t1 + counts.t2} ({counts.t1} vs {counts.t2})</p>

      <h6>Mutators</h6>
      <div class="mutators">
        {#each mutatorKeys as key}
          <div class="mrow">
            <span class="mkey">{cleanCase(key)}</span>
            <span class="mval">{mutatorLabel(key, tournament?.mutators?.[key])}</span>
          </div>
        {/each}
      </div>

      <div class="alert">
        <b>Important:</b> The host must configure mutators in Rocket Plugin to match the
        settings above. RLBot injects bots but cannot change match settings once the host
        has started the server.
      </div>
    </div>
  {/snippet}
  {#snippet footer()}
    <div class="footer">
      <button onclick={() => (visible = false)}>Close</button>
      <button
        class="info"
        onclick={() => {
          onOpenRL();
          visible = false;
        }}
      >
        🎮 Open Rocket League
      </button>
    </div>
  {/snippet}
</Modal>

<style>
  .body {
    font-size: 0.85rem;
    line-height: 1.4;
    max-width: 40rem;
  }
  .body h5 {
    margin: 0 0 0.4rem;
  }
  .body h6 {
    margin: 0.8rem 0 0.3rem;
    color: #00d9ff;
    text-transform: uppercase;
    font-size: 0.72rem;
    letter-spacing: 0.5px;
  }
  .body ol {
    margin: 0;
    padding-left: 1.2rem;
  }
  .settings {
    list-style: none;
    padding: 0;
    margin: 0;
  }
  .mutators {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 0.15rem 1rem;
  }
  .mrow {
    display: flex;
    justify-content: space-between;
    gap: 0.5rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
    padding: 0.1rem 0;
  }
  .mkey {
    color: #9fb0c0;
  }
  .mval {
    color: #fff;
    font-weight: 600;
    text-align: right;
  }
  .alert {
    margin-top: 0.8rem;
    padding: 0.5rem 0.7rem;
    border-radius: 0.4rem;
    background: rgba(0, 217, 255, 0.1);
    border: 1px solid rgba(0, 217, 255, 0.3);
    font-size: 0.78rem;
  }
  .footer {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
  }
  .footer button {
    padding: 0.3rem 0.7rem;
    border-radius: 0.3rem;
    cursor: pointer;
  }
  .footer button.info {
    background: #0a5a78;
    color: #fff;
  }
</style>
