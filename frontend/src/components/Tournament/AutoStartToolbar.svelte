<script lang="ts">
/**
 * Match-control toolbar for the live tournament (ported from v4 active.html's
 * match-control-toolbar). Holds the manual Start Match / Open Rocket League /
 * Human Match Info buttons and the auto-start controls (toggle, interval,
 * human-match behavior, countdown). Purely presentational: all state and logic
 * live in the parent (Tournament.svelte) so the auto-start scheduler has a
 * single owner.
 */
import NiceSelect from "../NiceSelect.svelte";

let {
  enabled = false,
  interval = $bindable(30),
  humanBehavior = $bindable("pause"),
  countdown = null,
  matchInProgress = false,
  selectedMatchId = "",
  onStart,
  onStop,
  onOpenRL,
  onShowInfo,
  onToggle,
}: {
  enabled?: boolean;
  interval?: number;
  humanBehavior?: string;
  countdown?: number | null;
  matchInProgress?: boolean;
  selectedMatchId?: string;
  onStart: (manual: boolean) => void;
  onStop: () => void;
  onOpenRL: () => void;
  onShowInfo: () => void;
  onToggle: () => void;
} = $props();

const intervalOptions = {
  Immediate: 0,
  "10 seconds": 10,
  "30 seconds": 30,
  "1 minute": 60,
  "2 minutes": 120,
};
const behaviorOptions = {
  "Start Match": "continue",
  "Pause Auto-Start": "pause",
  "Skip Match": "skip",
};
</script>

<div class="toolbar">
  <div class="left">
    {#if !matchInProgress}
      <button
        class="start"
        title="Start the selected match (or next available match)"
        onclick={() => onStart(true)}
      >
        ▶ Start Match
      </button>
      <button class="info" onclick={onOpenRL}>🎮 Open Rocket League</button>
      <button class="ghost" onclick={onShowInfo}>ℹ Human Match Info</button>
      {#if selectedMatchId}<span class="hint">✓ Match selected</span>{/if}
    {:else}
      <span class="inprogress">● Match in progress…</span>
      <button class="stop" title="Stop the running match" onclick={onStop}>
        ⏹ Stop Match
      </button>
    {/if}
  </div>

  <div class="right">
    <button
      class="toggle"
      class:on={enabled}
      onclick={onToggle}
      title="Toggle auto-start"
    >
      {enabled ? "⏸ Auto-Start ON" : "⏳ Auto-Start OFF"}
    </button>
    <label class="field">
      <span>Interval</span>
      <NiceSelect bind:value={interval} options={intervalOptions} />
    </label>
    <label class="field">
      <span>Human Matches</span>
      <NiceSelect bind:value={humanBehavior} options={behaviorOptions} />
    </label>
    {#if enabled && countdown !== null}
      <span class="countdown">⧗ Next match in: {countdown}s</span>
    {/if}
  </div>
</div>

<style>
  .toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
    padding: 0.5rem 0.6rem;
    margin-bottom: 0.75rem;
    border-radius: 0.4rem;
    background: rgba(0, 0, 0, 0.35);
    border: 1px solid rgba(255, 255, 255, 0.08);
  }
  .left,
  .right {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  button {
    font-size: 0.8rem;
    padding: 0.3rem 0.6rem;
    border-radius: 0.3rem;
    border: 1px solid transparent;
    cursor: pointer;
  }
  button.start {
    background: #15680e;
    color: #fff;
  }
  button.info {
    background: #0a5a78;
    color: #fff;
  }
  button.ghost {
    background: transparent;
    border-color: rgba(255, 255, 255, 0.25);
    color: var(--foreground, #eee);
  }
  .hint {
    font-size: 0.75rem;
    color: #7CFC9B;
  }
  .inprogress {
    color: #ffc107;
    font-weight: 600;
    font-size: 0.85rem;
  }
  button.stop {
    background: rgba(220, 53, 69, 0.85);
    border-color: rgba(220, 53, 69, 0.9);
    color: #fff;
  }
  .toggle {
    background: rgba(255, 255, 255, 0.08);
    color: var(--foreground, #eee);
  }
  .toggle.on {
    background: #15680e;
    color: #fff;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    color: #9fb0c0;
  }
  .field :global(select) {
    font-size: 0.78rem;
  }
  .countdown {
    font-size: 0.8rem;
    color: #00d9ff;
    font-weight: 600;
    white-space: nowrap;
  }
</style>
