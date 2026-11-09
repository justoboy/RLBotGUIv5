<script lang="ts">
  /** @import * from '../../../bindings/gui' */
  import { App } from "../../../bindings/gui/index.js";
  import toast from "svelte-5-french-toast";

  /**
   * Team roster panel, ported from the v4 fork's tournament "team panel"
   * (active.html). Shown above the bracket whenever team_size > 1. It lists
   * every team as a card (editable name + W-L record) and each member with its
   * in-game slot (S1, S2, ...) plus up/down arrows to reassign seats. The header
   * carries the actions that operate on teams WITHOUT touching participant
   * seeding: "Randomize Names" (fresh random team names, same members) and
   * "Re-form Teams" (re-shuffle members into new teams).
   *
   * The Go side is the source of truth: every mutation returns fresh state,
   * handed back via onUpdate so the bracket re-renders.
   */
  let {
    tournament,
    onUpdate,
  }: {
    tournament: any;
    onUpdate: (state: any) => void;
  } = $props();

  let teams = $derived(tournament?.teams || []);
  let teamSize = $derived(tournament?.team_size || 1);
  let completed = $derived(!!tournament?.completed);

  async function randomizeNames() {
    try {
      const state = await App.TournamentRandomizeTeamNames();
      onUpdate(state);
      toast.success("Team names randomized");
    } catch (e) {
      toast.error(`Failed to randomize names: ${e}`);
    }
  }

  async function reformTeams() {
    try {
      const state = await App.TournamentReformTeams();
      onUpdate(state);
      toast.success("Teams re-formed");
    } catch (e) {
      toast.error(`Failed to re-form teams: ${e}`);
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

  async function moveMember(teamIndex: number, fromSlot: number, toSlot: number) {
    try {
      const state = await App.TournamentReorderTeamMember(teamIndex, fromSlot, toSlot);
      onUpdate(state);
    } catch (e) {
      toast.error(`Failed to reorder member: ${e}`);
    }
  }
</script>

{#if teamSize > 1 && teams.length > 0}
  <div class="box blurred team-panel">
    <div class="team-panel-header">
      <h5>Teams ({teamSize}v{teamSize})</h5>
      <div class="team-panel-actions">
        <button onclick={randomizeNames} disabled={completed} title="Randomize team names (keeps members)">
          ⤨ Randomize Names
        </button>
        <button onclick={reformTeams} disabled={completed} title="Re-shuffle members into new teams">
          ⤨ Re-form Teams
        </button>
      </div>
    </div>

    <div class="team-grid">
      {#each teams as team, index (team.team_id)}
        <div class="team-card">
          <div class="team-card-name">
            <input
              type="text"
              maxlength={50}
              value={team.name}
              placeholder={`Team ${index + 1}`}
              disabled={completed}
              onchange={(e) => renameTeam(index, e.currentTarget.value)}
            />
            <span class="team-record">{team.wins}W - {team.losses}L</span>
          </div>
          <div class="team-card-members">
            {#each team.participants as p, slot (p.participant_id + ':' + slot)}
              <div class="team-card-member">
                <span class="slot">S{slot + 1}</span>
                <span class="type">{p.participant_type === "human" ? "👤" : "🤖"}</span>
                <span class="name">{p.name}</span>
                <span class="reorder">
                  <button
                    onclick={() => moveMember(index, slot, slot - 1)}
                    disabled={completed || slot === 0}
                    title="Move up"
                  >↑</button>
                  <button
                    onclick={() => moveMember(index, slot, slot + 1)}
                    disabled={completed || slot === team.participants.length - 1}
                    title="Move down"
                  >↓</button>
                </span>
              </div>
            {/each}
          </div>
        </div>
      {/each}
    </div>

    <p class="hint">
      Slot order sets the in-game seat (S1 = slot 1). Use the arrows to choose which
      player fills which seat.
    </p>
  </div>
{/if}

<style>
  .box {
    border-radius: 0.4rem;
    background-color: var(--background);
    padding: 0.6rem;
  }
  .team-panel {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    margin-bottom: 1rem;
  }
  .team-panel-header {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
  }
  .team-panel-header h5 {
    margin: 0;
    font-size: 0.95rem;
  }
  .team-panel-actions {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
  }
  .team-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
    gap: 0.5rem;
  }
  .team-card {
    border: 1px solid var(--background-alt, #333);
    border-radius: 0.4rem;
    padding: 0.45rem 0.55rem;
    background: var(--background-alt, #222);
  }
  .team-card-name {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin-bottom: 0.3rem;
  }
  .team-card-name input {
    flex: 1;
    min-width: 0;
    font-weight: 600;
  }
  .team-record {
    font-size: 0.7rem;
    opacity: 0.7;
    white-space: nowrap;
  }
  .team-card-members {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }
  .team-card-member {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.8rem;
  }
  .team-card-member .slot {
    font-weight: 700;
    opacity: 0.6;
    min-width: 1.4rem;
  }
  .team-card-member .name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .reorder {
    display: flex;
    gap: 0.1rem;
  }
  .reorder button {
    padding: 0 0.3rem;
    line-height: 1;
    font-size: 0.75rem;
  }
  .hint {
    font-size: 0.72rem;
    opacity: 0.6;
    margin: 0;
  }
</style>
