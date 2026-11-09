<script lang="ts">
/**
 * Tournament stats + match-history panel (ported from v4's stats table and
 * match-history view). Two tabs: a ranked per-entity stats table, and a
 * per-round expandable match history with per-player stat lines. Data is
 * fetched from the Go backend (TournamentGetStats / TournamentGetMatchHistory)
 * by the parent and passed in as props.
 */
let {
  stats,
  history,
  playerStats = [],
}: {
  stats: any;
  history: any;
  playerStats?: any[];
} = $props();

let tab = $state<"stats" | "players" | "history">("stats");
let expanded = $state<Record<string, boolean>>({});

function pct(r: number): string {
  return r == null ? "—" : `${Math.round(r * 100)}%`;
}
</script>

<div class="panel">
<div class="tabs">
  <button class:active={tab === "stats"} onclick={() => (tab = "stats")}>
    📊 {stats?.team_size > 1 ? "Team Stats" : "Stats"}
  </button>
  {#if stats?.team_size > 1}
    <button class:active={tab === "players"} onclick={() => (tab = "players")}>
      👤 Player Stats
    </button>
  {/if}
  <button class:active={tab === "history"} onclick={() => (tab = "history")}>
    🕘 Match History
  </button>
</div>

{#if tab === "players"}
  {#if playerStats?.length}
    <table class="stats-table">
      <thead>
        <tr>
          <th>Player</th>
          <th>Team</th>
          <th>Played</th>
          <th>Score</th>
          <th>Goals</th>
          <th>Assists</th>
          <th>Saves</th>
          <th>Shots</th>
          <th>Demolitions</th>
          <th>Own Goals</th>
        </tr>
      </thead>
      <tbody>
        {#each playerStats as ps}
          <tr>
            <td class="name">{ps.name}</td>
            <td>{ps.team || "—"}</td>
            <td>{ps.matches_played}</td>
            <td>{ps.score}</td>
            <td>{ps.goals}</td>
            <td>{ps.assists}</td>
            <td>{ps.saves}</td>
            <td>{ps.shots}</td>
            <td>{ps.demolitions}</td>
            <td>{ps.own_goals}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {:else}
    <p class="muted">No player stats recorded yet.</p>
  {/if}
{:else if tab === "stats"}
  {#if stats}
    <div class="summary">
      <span class="tag">{stats.format}</span>
      <span class="tag">{stats.completed_matches}/{stats.total_matches} played</span>
      <span class="tag">{stats.total_goals} goals</span>
      {#if stats.completed}<span class="tag win">Winner: {stats.winner_team || stats.winner}</span>{/if}
    </div>
    <table class="stats-table">
      <thead>
        <tr>
          <th>#</th>
          <th>Entity</th>
          <th>Played</th>
          <th>Wins</th>
          <th>Draws</th>
          <th>Losses</th>
          <th>Win Rate</th>
          <th>Goals For</th>
          <th>Goals Against</th>
          <th>Goal Difference</th>
          <th>Score</th>
          <th>Assists</th>
          <th>Saves</th>
          <th>Shots</th>
          <th>Demolitions</th>
          <th>Own Goals</th>
        </tr>
      </thead>
      <tbody>
        {#each stats.ranked || [] as row, i}
          <tr>
            <td>{i + 1}</td>
            <td class="name">{row.name}</td>
            <td>{row.matches_played}</td>
            <td>{row.wins}</td>
            <td>{row.draws}</td>
            <td>{row.losses}</td>
            <td>{pct(row.win_rate)}</td>
            <td>{row.goals_for}</td>
            <td>{row.goals_against}</td>
            <td class={row.goal_difference > 0 ? "pos" : row.goal_difference < 0 ? "neg" : ""}>
              {row.goal_difference > 0 ? "+" : ""}{row.goal_difference}
            </td>
            <td>{row.score}</td>
            <td>{row.assists}</td>
            <td>{row.saves}</td>
            <td>{row.shots}</td>
            <td>{row.demolitions}</td>
            <td>{row.own_goals}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {:else}
    <p class="muted">No stats available.</p>
  {/if}
{:else}
  {#if history}
    <div class="summary">
      <span class="tag">{history.completed_matches}/{history.total_matches} played</span>
      {#if history.completed}<span class="tag win">Winner: {history.winner_team || history.winner}</span>{/if}
    </div>
    {#each history.rounds || [] as rd}
      <div class="round-block">
        <div class="round-head">
          <span class="bracket-label" class:losers={rd.bracket === "losers"}>
            {rd.bracket === "losers" ? "LB" : rd.bracket === "winners" ? "WB" : "R"} {rd.round_num}
          </span>
        </div>
        {#each rd.matches as m}
          <div class="hist-match" class:done={m.completed}>
            <button class="hist-toggle" onclick={() => (expanded[m.match_id] = !expanded[m.match_id])}>
              <span class="t1">{m.team1_name || m.participant1_name || "TBD"}</span>
              <span class="score">{m.completed ? `${m.score?.[0]}–${m.score?.[1]}` : "vs"}</span>
              <span class="t2">{m.team2_name || m.participant2_name || "TBD"}</span>
              {#if m.is_bye}<span class="bye">BYE</span>{/if}
              {#if m.completed}<span class="w">🏆 {m.winner_name}</span>{/if}
              {#if m.has_duration}<span class="dur">{m.duration_seconds}s{m.is_overtime ? " OT" : ""}</span>{/if}
            </button>
            {#if expanded[m.match_id] && m.player_stats?.length}
              <table class="player-stats">
                <thead>
                  <tr>
                    <th>Player</th><th>Team</th><th>Score</th><th>Goals</th><th>Assists</th><th>Saves</th><th>Shots</th><th>Demolitions</th><th>Own Goals</th>
                  </tr>
                </thead>
                <tbody>
                  {#each m.player_stats as ps}
                    <tr>
                      <td class="name">{ps.name}</td>
                      <td>{ps.team === 0 ? "Blue" : "Orange"}</td>
                      <td>{ps.score}</td>
                      <td>{ps.goals}</td>
                      <td>{ps.assists}</td>
                      <td>{ps.saves}</td>
                      <td>{ps.shots}</td>
                      <td>{ps.demolitions}</td>
                      <td>{ps.own_goals}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/if}
          </div>
        {/each}
      </div>
    {/each}
  {:else}
    <p class="muted">No match history.</p>
  {/if}
{/if}
</div>

<style>
  .panel {
    margin-top: 0.75rem;
    padding: 0.75rem;
    border-radius: 0.5rem;
    background: rgba(10, 14, 20, 0.82);
    border: 1px solid rgba(255, 255, 255, 0.1);
    backdrop-filter: blur(2px);
  }
  .tabs {
    display: flex;
    gap: 0.4rem;
    margin-bottom: 0.6rem;
  }
  .tabs button {
    font-size: 0.78rem;
    padding: 0.25rem 0.6rem;
    border-radius: 0.3rem;
    background: rgba(255, 255, 255, 0.06);
    color: var(--foreground, #eee);
    border: 1px solid transparent;
    cursor: pointer;
  }
  .tabs button.active {
    background: rgba(0, 217, 255, 0.18);
    border-color: rgba(0, 217, 255, 0.4);
    color: #00d9ff;
  }
  .summary {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
    margin-bottom: 0.5rem;
  }
  .tag {
    background: var(--foreground, #eee);
    color: var(--background, #111);
    border-radius: 0.3rem;
    padding: 0.05rem 0.4rem;
    font-size: 0.72rem;
  }
  .tag.win {
    background: gold;
    color: #000;
  }
  .muted {
    color: #8a97a6;
    font-size: 0.85rem;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.76rem;
  }
  th,
  td {
    text-align: left;
    padding: 0.2rem 0.35rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  }
  th {
    color: #9fb0c0;
    text-transform: uppercase;
    font-size: 0.62rem;
    letter-spacing: 0.5px;
  }
  td.name {
    font-weight: 600;
    color: #fff;
  }
  td.pos {
    color: #4ade80;
  }
  td.neg {
    color: #f87171;
  }
  .round-block {
    margin-bottom: 0.6rem;
  }
  .round-head {
    margin-bottom: 0.25rem;
  }
  .bracket-label {
    font-size: 0.68rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: #00d9ff;
  }
  .bracket-label.losers {
    color: #ffc107;
  }
  .hist-match {
    margin-bottom: 0.3rem;
  }
  .hist-toggle {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    text-align: left;
    background: rgba(0, 0, 0, 0.3);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 0.3rem;
    padding: 0.3rem 0.5rem;
    color: #fff;
    cursor: pointer;
    font-size: 0.78rem;
  }
  .hist-match.done .hist-toggle {
    border-color: rgba(40, 167, 69, 0.5);
  }
  .hist-toggle .score {
    font-weight: 700;
    color: #cdd6e0;
  }
  .hist-toggle .t1 {
    color: #6ab0ff;
  }
  .hist-toggle .t2 {
    color: #ffd27a;
  }
  .hist-toggle .w {
    margin-left: auto;
    color: gold;
    font-size: 0.72rem;
  }
  .hist-toggle .bye {
    color: #8a97a6;
    font-size: 0.65rem;
  }
  .hist-toggle .dur {
    color: #8a97a6;
    font-size: 0.65rem;
  }
  .player-stats {
    margin: 0.25rem 0 0.25rem 0.5rem;
    width: calc(100% - 0.5rem);
  }
</style>
