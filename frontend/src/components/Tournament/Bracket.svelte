<script lang="ts">
/**
 * Tournament bracket renderer (ported from v4 tournament-vue.js).
 *
 * Renders a "butterfly" single/double-elimination bracket: the winners bracket
 * flows from the outer edges inward to a centered Finals match, and (for double
 * elimination) a separate Losers Bracket section sits below. SVG elbow
 * connectors are drawn between matches using each match's next_match_id /
 * loser_next_match_id links so it is obvious which match feeds into another.
 *
 * Round-robin and Swiss formats have no bracket tree, so they render as a flat
 * list of rounds instead.
 */
import { tick } from "svelte";

let {
  tournament,
  startingMatchId = "",
  selectedMatchId = $bindable(""),
  onPlay,
  onRecord,
}: {
  tournament: any;
  startingMatchId?: string;
  selectedMatchId?: string;
  onPlay: (match: any, useStaging: boolean) => void;
  onRecord: (match: any, winnerName: string) => void;
} = $props();
let wbSvg: SVGSVGElement | undefined = $state();
let lbSvg: SVGSVGElement | undefined = $state();
let wbSection: HTMLElement | undefined = $state();
let lbSection: HTMLElement | undefined = $state();

let format = $derived(tournament?.format);
let isDouble = $derived(format === "double_elimination");
let isFlat = $derived(format === "round_robin" || format === "swiss");

// Winners-bracket matches (W-prefixed for double elim, all for single elim).
let wbMatches = $derived.by(() => {
  const all = tournament?.matches || [];
  return isDouble ? all.filter((m: any) => String(m.match_id).startsWith("W")) : all;
});

// Losers-bracket matches (LB array + the L-prefixed Grand Final for double elim).
let lbMatches = $derived.by(() => {
  if (!isDouble) return [];
  const lb = tournament?.losers_bracket_matches || [];
  const gf = (tournament?.matches || []).filter((m: any) => String(m.match_id).startsWith("L"));
  return [...lb, ...gf];
});

// --- layout (filled in via later edits) ---
let butterfly = $derived.by(() => groupButterfly(wbMatches));
let wbPositions = $derived.by(() => computeBracketPositions(butterfly, wbMatches));
let lbByRound = $derived.by(() => groupByRound(lbMatches));
let lbPositions = $derived.by(() => computeLosersBracketPositions(lbMatches, wbPositions));
let flatRounds = $derived.by(() => groupByRound(tournament?.matches || []));
let currentRound = $derived.by(() => findCurrentRound(tournament?.matches || []));

// Dynamic column height. Butterfly cards are absolutely positioned with top:X% inside the
// .bracket-round-matches column. Round-1 cards are spread at spacing*(i+1) where
// spacing = 100/(n+1), so the vertical gap between two stacked cards is height/(n+1). As more
// bots are added, a column's match count n grows and that gap shrinks below the card height,
// causing overlap. To guarantee gap >= slot we need height/(n+1) >= slot, i.e. height >=
// (n+1)*slot. The WB slot is one card tall (plus a little slack). The LB needs MORE: each LB
// round-2 match sits between two round-1 cards, so its column reserves a full extra card height
// per slot (WB slot + one card) to guarantee an R2 card fits in the gap between R1 cards.
const WB_CARD_SLOT = 116; // px reserved per stacked WB card (card ~108px: id row + two bars + Play)
// LB cards sit closer together than WB cards. WB_CARD_SLOT + 104 (220) was slightly too loose;
// adding a little less than 104 (96) packs them tighter without overlap.
const LB_CARD_SLOT = WB_CARD_SLOT + 96; // px reserved per stacked LB card
let wbMaxPerColumn = $derived.by(() => {
  let max = 1;
  for (const wr of butterfly.left) max = Math.max(max, wr.matches.length);
  for (const wr of butterfly.right) max = Math.max(max, wr.matches.length);
  if (butterfly.center) max = Math.max(max, butterfly.center.matches.length);
  return max;
});
let lbMaxPerColumn = $derived.by(() => {
  let max = 1;
  for (const rd of lbByRound) max = Math.max(max, rd.matches.length);
  return max;
});
let wbMinHeight = $derived((wbMaxPerColumn + 1) * WB_CARD_SLOT + 24);
let lbMinHeight = $derived((lbMaxPerColumn + 1) * LB_CARD_SLOT + 24);

// Single connector color per section (color-coding was too noisy and removed).
const WB_CONNECTOR_COLOR = "rgba(0, 217, 255, 0.45)";
const LB_CONNECTOR_COLOR = "rgba(255, 193, 7, 0.45)";

// Redraw connectors whenever the rendered matches change.
$effect(() => {
  void tournament;
  tick().then(() => drawAllConnectors());
});

// --- helpers (stubs filled in via later edits) ---
function groupByRound(matches: any[]) {
  // Group matches by round_num, sorted ascending. Returns [{roundNum, matches}].
  const byRound: Record<number, any[]> = {};
  for (const m of matches) (byRound[m.round_num] ||= []).push(m);
  return Object.keys(byRound)
    .map(Number)
    .sort((a, b) => a - b)
    .map((rn) => ({ roundNum: rn, matches: byRound[rn] }));
}
function groupButterfly(matches: any[]) {
  // Split a bracket into {left, center, right} wings for the butterfly layout.
  // Rounds with >1 match split into a left half and right half; the single-match
  // final round becomes the center. Right wing is reversed (finals -> round 1).
  const byRound: Record<number, any[]> = {};
  for (const m of matches) (byRound[m.round_num] ||= []).push(m);
  const roundNums = Object.keys(byRound).map(Number).sort((a, b) => a - b);
  const left: { roundNum: number; matches: any[] }[] = [];
  const right: { roundNum: number; matches: any[] }[] = [];
  let center: { roundNum: number; matches: any[] } | null = null;
  for (const rn of roundNums) {
    const ms = byRound[rn];
    if (ms.length === 1) {
      center = { roundNum: rn, matches: ms };
    } else {
      const half = Math.ceil(ms.length / 2);
      left.push({ roundNum: rn, matches: ms.slice(0, half) });
      right.push({ roundNum: rn, matches: ms.slice(half) });
    }
  }
  right.reverse();
  return { left, center, right };
}
function computeBracketPositions(bfly: any, allMatches: any[]) {
  // Vertical % position per match. Each wing is positioned independently across
  // 0-100%: its round-1 matches spread evenly, later rounds at the midpoint of
  // their feeders (matches whose next_match_id points here). Center at 50%.
  const positions: Record<string, number> = {};
  const { left, center, right } = bfly;
  const positionWing = (wingRounds: any[]) => {
    if (!wingRounds || wingRounds.length === 0) return;
    let r1 = wingRounds[0];
    for (const r of wingRounds) if (r.matches.length > r1.matches.length) r1 = r;
    const n = r1.matches.length;
    const spacing = 100 / (n + 1);
    r1.matches.forEach((m: any, i: number) => (positions[m.match_id] = spacing * (i + 1)));
    const sorted = [...wingRounds].sort((a, b) => a.roundNum - b.roundNum);
    for (const round of sorted) {
      if (round.roundNum === r1.roundNum) continue;
      for (const m of round.matches) {
        if (positions[m.match_id] !== undefined) continue;
        const feeders = allMatches.filter((f) => f.next_match_id === m.match_id);
        if (feeders.length > 0) {
          const sum = feeders.reduce(
            (acc, f) => acc + (positions[f.match_id] !== undefined ? positions[f.match_id] : 50),
            0,
          );
          positions[m.match_id] = sum / feeders.length;
        } else {
          positions[m.match_id] = 50;
        }
      }
    }
  };
  positionWing(left);
  positionWing(right);
  if (center) for (const m of center.matches) positions[m.match_id] = 50;
  return positions;
}
function findCurrentRound(matches: any[]) {
  // First round with an incomplete match; else the last round.
  const byRound: Record<number, any[]> = {};
  for (const m of matches) (byRound[m.round_num] ||= []).push(m);
  const roundNums = Object.keys(byRound).map(Number).sort((a, b) => a - b);
  for (const rn of roundNums) {
    if (byRound[rn].some((m) => !m.completed)) return rn;
  }
  return roundNums.length ? roundNums[roundNums.length - 1] : 1;
}
function computeLosersBracketPositions(matches: any[], wbPositions: Record<string, number>) {
  // Vertical % positions for the losers bracket. Each LB round is its own column and
  // its matches are spread EVENLY across the full height (exactly like a WB wing round),
  // which guarantees no column ever overlaps no matter how round sizes grow/shrink. To
  // keep connectors from crossing, matches within a round are sorted by the vertical
  // position of the match that feeds them: an LB-fed match sorts by its LB feeder's
  // position, a WB-fed "entry point" sorts by its WB feeder's position. Preserving the
  // feeder order top-to-bottom means a higher feeder always feeds a higher match, so the
  // elbow connectors never cross. (The old chain-copy approach dumped every WB-fed entry
  // point into one center gap, and that pile-up propagated down the chain into the deep
  // rounds, overlapping the last few LB matches.)
  const positions: Record<string, number> = {};
  if (!matches || matches.length === 0) return positions;
  const allMatches = tournament?.matches || [];
  const roundNums = [...new Set(matches.map((m) => m.round_num))].sort((a, b) => a - b);
  const byRound: Record<number, any[]> = {};
  for (const rn of roundNums) byRound[rn] = matches.filter((m) => m.round_num === rn);
  const sortKey = (m: any): number => {
    const lbF = matches.find((f) => f.next_match_id === m.match_id && f.round_num < m.round_num);
    if (lbF && positions[lbF.match_id] !== undefined) return positions[lbF.match_id];
    const wbF = allMatches.find((f: any) => f.loser_next_match_id === m.match_id);
    if (wbF && wbPositions[wbF.match_id] !== undefined) return wbPositions[wbF.match_id];
    return 50;
  };
  for (const rn of roundNums) {
    const roundMatches = [...byRound[rn]].sort((a, b) => sortKey(a) - sortKey(b));
    const n = roundMatches.length;
    roundMatches.forEach((m, i) => (positions[m.match_id] = ((i + 0.5) / n) * 100));
  }
  return positions;
}
function isMatchActive(m: any) {
  return !m.completed && !!m.participant1 && !!m.participant2;
}
function isMatchInProgress(m: any) {
  return startingMatchId === m.match_id;
}
function getRoundName(roundNum: number, bracket?: "winners" | "losers") {
  // Name rounds by position from the END (Finals, Semi-Finals, Quarter-Finals).
  // For double elim, the WB's last round is "Semi-Finals" (it feeds the GF).
  if (!tournament) return `Round ${roundNum}`;
  const fmt = tournament.format;
  if (fmt === "single_elimination") {
    const all = (tournament.matches || []).map((m: any) => m.round_num);
    if (!all.length) return `Round ${roundNum}`;
    const total = Math.max(...all);
    return nameFromEnd(total - roundNum + 1, roundNum);
  }
  if (fmt === "double_elimination") {
    if (bracket === "losers") {
      // Name by position from the END, over the SAME set the losers section
      // renders (LB matches + the L-prefixed Grand Final). Using only
      // losers_bracket_matches here would exclude the GF and mislabel the
      // columns (e.g. 4-bot: Semi->Finals->Round 3 instead of
      // Quarter->Semi->Finals).
      const lb = lbMatches;
      if (!lb.length) return `Round ${roundNum}`;
      const total = Math.max(...lb.map((m: any) => m.round_num));
      return nameFromEnd(total - roundNum + 1, roundNum);
    }
    const wb = (tournament.matches || []).filter((m: any) => String(m.match_id).startsWith("W"));
    if (!wb.length) return `Round ${roundNum}`;
    const total = Math.max(...wb.map((m: any) => m.round_num));
    return roundNum === total ? "Semi-Finals" : `Round ${roundNum}`;
  }
  if (fmt === "swiss") return `Round ${roundNum} of ${tournament.swiss_rounds || 0}`;
  return `Round ${roundNum}`;
}
function nameFromEnd(roundsFromEnd: number, roundNum: number) {
  if (roundsFromEnd === 1) return "Finals";
  if (roundsFromEnd === 2) return "Semi-Finals";
  if (roundsFromEnd === 3) return "Quarter-Finals";
  return `Round ${roundNum}`;
}
function selectMatch(m: any) {
  if (m.completed || !m.participant1 || !m.participant2) return;
  selectedMatchId = selectedMatchId === m.match_id ? "" : m.match_id;
}
function drawAllConnectors() {
  // Draw elbow connectors into each section's SVG overlay. Coordinates are
  // relative to each section's scroll container so they scroll with content.
  // Each section uses a single uniform connector color (WB cyan, LB amber).
  if (wbSvg && wbSection) {
    const sc = wbSection.querySelector(".bracket-rounds") as HTMLElement | null;
    if (sc) drawSectionConnectors(wbSvg, sc, wbMatches, WB_CONNECTOR_COLOR);
  }
  if (lbSvg && lbSection && lbMatches.length) {
    const sc = lbSection.querySelector(".bracket-rounds") as HTMLElement | null;
    if (sc) drawSectionConnectors(lbSvg, sc, lbMatches, LB_CONNECTOR_COLOR);
  }
}
function drawSectionConnectors(svg: SVGSVGElement, sc: HTMLElement, matches: any[], stroke: string) {
  const w = Math.max(sc.scrollWidth, sc.clientWidth);
  const h = Math.max(sc.scrollHeight, sc.clientHeight);
  svg.setAttribute("width", String(w));
  svg.setAttribute("height", String(h));
  svg.setAttribute("viewBox", `0 0 ${w} ${h}`);
  svg.style.left = "0";
  svg.style.top = "0";
  while (svg.firstChild) svg.removeChild(svg.firstChild);
  const byId: Record<string, any> = {};
  for (const m of matches) byId[m.match_id] = m;
  const rect = sc.getBoundingClientRect();
  const originX = rect.left;
  const originY = rect.top;
  for (const m of matches) {
    if (!m.next_match_id) continue;
    const target = byId[m.next_match_id];
    if (!target) continue;
    const srcEl = sc.querySelector(`[data-match-id="${m.match_id}"]`);
    const dstEl = sc.querySelector(`[data-match-id="${target.match_id}"]`);
    if (!srcEl || !dstEl) continue;
    const s = srcEl.getBoundingClientRect();
    const d = dstEl.getBoundingClientRect();
    const srcCenterX = s.left + s.width / 2;
    const dstCenterX = d.left + d.width / 2;
    const srcIsLeft = srcCenterX < dstCenterX;
    const gap = 4;
    const x1 = srcIsLeft ? s.right - originX + gap : s.left - originX - gap;
    const y1 = s.top + s.height / 2 - originY;
    const x2 = srcIsLeft ? d.left - originX - gap : d.right - originX + gap;
    const y2 = d.top + d.height / 2 - originY;
    const midX = x1 + (x2 - x1) / 2;
    appendSvgLine(svg, x1, y1, midX, y1, stroke);
    appendSvgLine(svg, midX, y1, midX, y2, stroke);
    appendSvgLine(svg, midX, y2, x2, y2, stroke);
  }
}
function appendSvgLine(svg: SVGSVGElement, x1: number, y1: number, x2: number, y2: number, stroke: string) {
  const line = document.createElementNS("http://www.w3.org/2000/svg", "line");
  line.setAttribute("x1", x1.toFixed(1));
  line.setAttribute("y1", y1.toFixed(1));
  line.setAttribute("x2", x2.toFixed(1));
  line.setAttribute("y2", y2.toFixed(1));
  line.setAttribute("stroke", stroke);
  line.setAttribute("stroke-width", "2");
  line.setAttribute("stroke-linecap", "round");
  svg.appendChild(line);
}
</script>

{#if isFlat}
  <!-- Flat list (round robin / swiss): one column per round -->
  <div class="bracket-tree flat">
    {#each flatRounds as rd}
      <div class="bracket-round">
        <h6 class="round-title">{getRoundName(rd.roundNum)}</h6>
        <div class="bracket-round-matches static">
          {#each rd.matches as m}
            {@render card(m, 0, true)}
          {/each}
        </div>
      </div>
    {/each}
  </div>
{:else}
  <div class="bracket-tree">
    <!-- Winners Bracket -->
    <div class="bracket-section" bind:this={wbSection}>
      <h5 class="section-title">Winners Bracket</h5>
      <div class="bracket-rounds bracket-rounds-butterfly" style="min-height:{wbMinHeight}px">
        <svg class="bracket-connectors" bind:this={wbSvg} aria-hidden="true"></svg>
        {#each butterfly.left as wr}
          <div class="bracket-round butterfly-wing-left" class:current-round={wr.roundNum === currentRound}>
            <h6 class="round-title">{getRoundName(wr.roundNum, "winners")}</h6>
            <div class="bracket-round-matches">
              {#each wr.matches as m}
                {@render card(m, wbPositions[m.match_id] ?? 50, false)}
              {/each}
            </div>
          </div>
        {/each}
        {#if butterfly.center}
          <div class="bracket-round butterfly-center">
            <h6 class="round-title">{getRoundName(butterfly.center.roundNum, "winners")}</h6>
            <div class="bracket-round-matches">
              {#each butterfly.center.matches as m}
                {@render card(m, wbPositions[m.match_id] ?? 50, false)}
              {/each}
            </div>
          </div>
        {/if}
        {#each butterfly.right as wr}
          <div class="bracket-round butterfly-wing-right" class:current-round={wr.roundNum === currentRound}>
            <h6 class="round-title">{getRoundName(wr.roundNum, "winners")}</h6>
            <div class="bracket-round-matches">
              {#each wr.matches as m}
                {@render card(m, wbPositions[m.match_id] ?? 50, false)}
              {/each}
            </div>
          </div>
        {/each}
      </div>
    </div>

    <!-- Losers Bracket (double elimination) -->
    {#if isDouble && lbByRound.length}
      <div class="bracket-section" bind:this={lbSection}>
        <h5 class="section-title losers-title">Losers Bracket</h5>
        <div class="bracket-rounds" style="min-height:{lbMinHeight}px">
          <svg class="bracket-connectors" bind:this={lbSvg} aria-hidden="true"></svg>
          {#each lbByRound as rd}
            <div class="bracket-round">
              <h6 class="round-title">{getRoundName(rd.roundNum, "losers")}</h6>
              <div class="bracket-round-matches">
                {#each rd.matches as m}
                  {@render card(m, lbPositions[m.match_id] ?? 50, false)}
                {/each}
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  </div>
{/if}

{#snippet card(m: any, pos: number, flat: boolean)}
  <div
    class="bracket-match"
    class:flat
    class:butterfly-match={!flat}
    class:completed={m.completed}
    class:active={isMatchActive(m)}
    class:in-progress={isMatchInProgress(m)}
    class:selected={selectedMatchId === m.match_id}
    style={flat ? "" : `top:${pos}%`}
    data-match-id={m.match_id}
    onclick={() => selectMatch(m)}
    role="button"
    tabindex={0}
    onkeydown={(e) => e.key === "Enter" && selectMatch(m)}
  >
    <div class="match-id-row">
      <span class="match-id" title="Match ID">{m.match_id}</span>
      {#if m.next_match_id}<span class="match-id target" title="Feeds match">{m.next_match_id}</span>{/if}
    </div>
    <div class="match-team-bars">
      <div
        class="team-bar team-bar-1"
        class:winner={m.completed && m.winner && m.winner.name === m.participant1?.name}
        class:loser={m.completed && m.winner && m.winner.name !== m.participant1?.name}
        class:tbd={!m.participant1}
      >
        <span class="team-bar-name">{m.participant1?.name || "TBD"}</span>
        {#if m.score}<span class="team-bar-score">{m.score[0]}</span>{/if}
      </div>
      <div
        class="team-bar team-bar-2"
        class:winner={m.completed && m.winner && m.winner.name === m.participant2?.name}
        class:loser={m.completed && m.winner && m.winner.name !== m.participant2?.name}
        class:tbd={!m.participant2}
      >
        <span class="team-bar-name">{m.participant2?.name || "TBD"}</span>
        {#if m.score}<span class="team-bar-score">{m.score[1]}</span>{/if}
      </div>
    </div>
    {#if !m.completed}
      <div class="matchActions">
        <button
          disabled={!m.participant1 || !m.participant2}
          onclick={(e) => { e.stopPropagation(); onPlay(m, false); }}
        >
          Play
        </button>
      </div>
    {:else}
      <div class="done">✓ {m.winner?.name}</div>
    {/if}
  </div>
{/snippet}

<style>
  .bracket-tree {
    display: flex;
    flex-direction: column;
    gap: 20px;
    overflow-x: hidden;
    overflow-y: auto;
    position: relative;
  }
  .bracket-tree.flat {
    flex-direction: row;
    flex-wrap: wrap;
    align-items: flex-start;
  }
  .bracket-section {
    display: flex;
    flex-direction: column;
    width: 100%;
    position: relative;
    overflow: hidden;
  }
  .section-title {
    color: #00d9ff;
    margin: 0 0 8px;
    font-size: 0.9rem;
    text-transform: uppercase;
    letter-spacing: 1px;
    align-self: flex-start;
    padding: 3px 12px;
    border-radius: 999px;
    background: rgba(0, 0, 0, 0.6);
    border: 1px solid rgba(0, 217, 255, 0.35);
    backdrop-filter: blur(2px);
  }
  .losers-title {
    color: #ffc107;
    border-color: rgba(255, 193, 7, 0.4);
  }
  .bracket-rounds {
    display: flex;
    flex-direction: row;
    align-items: stretch;
    flex: 1;
    gap: 30px;
    overflow-x: auto;
    padding: 12px 6px 16px;
    position: relative;
    min-height: 400px;
  }
  .bracket-rounds-butterfly {
    gap: 40px;
  }
  .bracket-round {
    position: relative;
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 150px;
  }
  .butterfly-wing-left,
  .butterfly-wing-right {
    flex: 1;
    min-width: 130px;
  }
  .butterfly-center {
    flex: 1.5;
    min-width: 150px;
  }
  .round-title {
    flex-shrink: 0;
    text-align: center;
    margin: 0 auto 14px;
    color: #cdd6e0;
    font-size: 0.72rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 1.5px;
    padding: 2px 10px;
    border-radius: 999px;
    background: rgba(0, 0, 0, 0.6);
    border: 1px solid rgba(255, 255, 255, 0.12);
    backdrop-filter: blur(2px);
    width: fit-content;
  }
  .bracket-round-matches {
    flex: 1;
    display: flex;
    flex-direction: column;
    justify-content: space-evenly;
    gap: 14px;
    min-height: 360px;
    position: relative;
    overflow: visible;
  }
  .bracket-round-matches.static {
    justify-content: flex-start;
  }
  .bracket-match {
    width: 100%;
    padding: 7px;
    background: rgba(0, 0, 0, 0.4);
    border: 1px solid rgba(0, 217, 255, 0.18);
    border-radius: 8px;
    position: relative;
    z-index: 2;
    cursor: pointer;
    transition: all 0.2s ease;
  }
  .bracket-match.flat {
    position: static;
  }
  .bracket-match.completed {
    opacity: 0.7;
    border-color: #28a745;
  }
  .bracket-match.active {
    border-color: #00d9ff;
  }
  .bracket-match.in-progress {
    border-color: #ffc107;
    background: rgba(255, 193, 7, 0.1);
  }
  .bracket-match.selected {
    border-color: #00ff88;
  }
  .butterfly-match {
    position: absolute;
    left: 0;
    width: 100%;
    transform: translateY(-50%);
    transition: top 0.3s ease;
  }
  .bracket-connectors {
    position: absolute;
    top: 0;
    left: 0;
    pointer-events: none;
    z-index: 0;
  }
  .match-team-bars {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .team-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 5px 10px;
    border-radius: 4px;
    font-size: 0.82rem;
    font-weight: 600;
    color: #fff;
    transition: all 0.2s ease;
  }
  .team-bar-1 {
    background: linear-gradient(90deg, rgba(0, 120, 255, 0.85), rgba(0, 120, 255, 0.55));
    border-left: 3px solid #0078ff;
  }
  .team-bar-2 {
    background: linear-gradient(90deg, rgba(255, 190, 0, 0.85), rgba(255, 190, 0, 0.55));
    border-left: 3px solid #ffbe00;
    color: #1a1a2e;
  }
  .team-bar.winner {
    box-shadow: 0 0 8px rgba(255, 255, 255, 0.35);
    filter: brightness(1.15);
  }
  .team-bar.loser,
  .team-bar.tbd {
    opacity: 0.35;
    filter: grayscale(0.6);
  }
  .team-bar-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .team-bar-score {
    margin-left: 8px;
    font-weight: bold;
  }
  .matchActions {
    display: flex;
    gap: 0.2rem;
    margin-top: 0.3rem;
  }
  .matchActions button {
    flex: 1;
    font-size: 0.72rem;
    padding: 0.15rem 0.3rem;
  }
  .done {
    margin-top: 0.3rem;
    font-size: 0.72rem;
    color: gold;
  }
  .match-id-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 6px;
    margin-bottom: 4px;
    font-size: 0.62rem;
    line-height: 1;
    letter-spacing: 0.5px;
    pointer-events: none;
  }
  .match-id {
    color: rgba(255, 255, 255, 0.45);
    font-weight: 700;
    font-variant-numeric: tabular-nums;
  }
  .match-id.target {
    color: rgba(0, 217, 255, 0.7);
  }
</style>
