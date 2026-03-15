import type { ApiGameState, ApiPlayerState } from "./types";
import type { GameEvent } from "./api";

/**
 * ReplayFrame: a snapshot of the visible game state at a point in time,
 * plus metadata about what just happened.
 */
export interface ReplayFrame {
  state: ApiGameState;
  /** Human-readable label for what just happened */
  label: string;
  /** The event type that produced this frame */
  eventType: string;
  /** Seat index of the player who just acted (for reason tooltip) */
  actionSeat?: number;
  /** AI decision reason from the last player action */
  reason?: string;
  /** Winners from pot_awarded event (for animation) */
  potAwardWinners?: { seat: number; amount: number }[];
}

// Number of seats in a standard poker game
const NUM_SEATS = 6;
const STARTING_CHIPS = 1500;

/**
 * Build an array of replay frames from raw game events.
 * Each "interesting" event produces one frame showing the state after that event.
 */
export function buildReplayFrames(
  gameId: string,
  events: GameEvent[],
  playerNames: Record<string, string>,
  playerAvatars?: Record<string, string>,
): ReplayFrame[] {
  const frames: ReplayFrame[] = [];

  // Running state
  const players: ApiPlayerState[] = [];
  let handNum = 0;
  let dealerSeat = 0;
  let smallBlind = 40;
  let bigBlind = 80;
  let phase = "idle";
  let community: string[] = [];
  let currentBet = 0;
  let actionOn = -1;
  let finished = false;
  let initialized = false;

  // Per-round bet tracking (reset each betting round)
  const playerBets: number[] = new Array(NUM_SEATS).fill(0);
  const playerTotalBets: number[] = new Array(NUM_SEATS).fill(0);

  function snapshot(label: string, eventType: string, action?: { seat: number; reason?: string }) {
    frames.push({
      state: {
        game_id: gameId,
        hand_num: handNum,
        phase,
        finished,
        community: [...community],
        current_bet: currentBet,
        small_blind: smallBlind,
        big_blind: bigBlind,
        dealer_seat: dealerSeat,
        pots: computePots(),
        action_on: actionOn,
        players: players.map((p) => ({ ...p, hole: p.hole ? [...p.hole] : null })),
      },
      label,
      eventType,
      actionSeat: action?.seat,
      reason: action?.reason,
    });
  }

  function computePots(): { amount: number; eligible: number[] }[] {
    const totalPot = playerTotalBets.reduce((a, b) => a + b, 0);
    if (totalPot <= 0) return [];
    const eligible = players
      .filter((p) => !p.folded && !p.eliminated)
      .map((p) => p.seat);
    return [{ amount: totalPot, eligible }];
  }

  function getPlayerName(id: string, seat: number): string {
    return playerNames[id] || `Player ${seat + 1}`;
  }

  function resetBettingRound() {
    for (let i = 0; i < NUM_SEATS; i++) {
      playerBets[i] = 0;
    }
    currentBet = 0;
    for (const p of players) {
      p.bet = 0;
    }
  }

  for (const evt of events) {
    const p = evt.payload;

    switch (evt.event_type) {
      case "hand_start": {
        // Initialize / reset for new hand
        const evtPlayers = p.players as { id: string; seat: number; chips: number }[];
        handNum = (p.hand_num as number) || handNum + 1;
        dealerSeat = (p.dealer_seat as number) ?? dealerSeat;
        smallBlind = (p.small_blind as number) || 40;
        bigBlind = (p.big_blind as number) || 80;
        phase = "preflop";
        community = [];
        currentBet = 0;
        actionOn = -1;
        playerBets.fill(0);
        playerTotalBets.fill(0);

        if (!initialized) {
          // First hand — create player entries
          for (let i = 0; i < NUM_SEATS; i++) {
            players[i] = {
              id: "",
              name: "",
              seat: i,
              chips: STARTING_CHIPS,
              bet: 0,
              total_bet: 0,
              hole: null,
              folded: false,
              all_in: false,
              eliminated: true,
            };
          }
          initialized = true;
        }

        // Update from event data
        for (const ep of evtPlayers) {
          const pl = players[ep.seat];
          pl.id = ep.id;
          pl.name = getPlayerName(ep.id, ep.seat);
          pl.avatar_url = playerAvatars?.[ep.id];
          pl.chips = ep.chips;
          pl.folded = false;
          pl.all_in = false;
          pl.hole = null;
          pl.bet = 0;
          pl.total_bet = 0;
          pl.eliminated = false;
        }

        // Mark seats not in this hand as eliminated
        const activeSeatSet = new Set(evtPlayers.map((ep) => ep.seat));
        for (const pl of players) {
          if (!activeSeatSet.has(pl.seat)) {
            pl.eliminated = true;
            pl.hole = null;
          }
        }

        snapshot(`Hand #${handNum}`, evt.event_type);
        break;
      }

      case "blinds_posted": {
        const sbSeat = p.sb_seat as number;
        const bbSeat = p.bb_seat as number;
        const sbAmt = p.sb_amount as number;
        const bbAmt = p.bb_amount as number;

        if (players[sbSeat]) {
          players[sbSeat].chips -= sbAmt;
          players[sbSeat].bet = sbAmt;
          playerBets[sbSeat] = sbAmt;
          playerTotalBets[sbSeat] = sbAmt;
          players[sbSeat].total_bet = sbAmt;
          if (players[sbSeat].chips <= 0) players[sbSeat].all_in = true;
        }
        if (players[bbSeat]) {
          players[bbSeat].chips -= bbAmt;
          players[bbSeat].bet = bbAmt;
          playerBets[bbSeat] = bbAmt;
          playerTotalBets[bbSeat] = bbAmt;
          players[bbSeat].total_bet = bbAmt;
          if (players[bbSeat].chips <= 0) players[bbSeat].all_in = true;
        }
        currentBet = bbAmt;
        // Don't snapshot — combined with hole_dealt
        break;
      }

      case "hole_dealt": {
        const seat = p.seat as number;
        const cards = p.cards as string[];
        if (players[seat]) {
          players[seat].hole = cards;
        }
        // Snapshot only after all holes are dealt (last non-eliminated seat)
        const allDealt = players.every(
          (pl) => pl.eliminated || (pl.hole && pl.hole.length === 2),
        );
        if (allDealt) {
          snapshot(`Cards dealt — Hand #${handNum}`, evt.event_type);
        }
        break;
      }

      case "player_action": {
        const seat = p.seat as number;
        const action = p.action as string;
        const amount = p.amount as number;
        const chipsLeft = p.chips_left as number;
        const reason = p.reason as string | undefined;
        const pl = players[seat];
        if (!pl) break;

        // Infer initial state from first action if no hand_start seen
        if (!initialized) {
          initialized = true;
          for (let i = 0; i < NUM_SEATS; i++) {
            players[i] = players[i] || {
              id: "",
              name: "",
              seat: i,
              chips: STARTING_CHIPS,
              bet: 0,
              total_bet: 0,
              hole: null,
              folded: false,
              all_in: false,
              eliminated: false,
            };
          }
          handNum = 1;
          phase = "preflop";
        }

        actionOn = seat;
        const name = pl.name || getPlayerName(pl.id, seat);
        const act = { seat, reason };

        switch (action) {
          case "fold":
            pl.folded = true;
            snapshot(`${name} folds`, evt.event_type, act);
            break;
          case "check":
            snapshot(`${name} checks`, evt.event_type, act);
            break;
          case "call":
            pl.chips = chipsLeft;
            pl.bet += amount;
            pl.total_bet += amount;
            playerBets[seat] += amount;
            playerTotalBets[seat] += amount;
            snapshot(`${name} calls $${amount}`, evt.event_type, act);
            break;
          case "raise":
            pl.chips = chipsLeft;
            pl.bet += amount;
            pl.total_bet += amount;
            playerBets[seat] += amount;
            playerTotalBets[seat] += amount;
            currentBet = pl.bet;
            snapshot(`${name} raises to $${pl.bet}`, evt.event_type, act);
            break;
          case "allin":
            pl.chips = 0;
            pl.all_in = true;
            pl.bet += amount;
            pl.total_bet += amount;
            playerBets[seat] += amount;
            playerTotalBets[seat] += amount;
            if (pl.bet > currentBet) currentBet = pl.bet;
            snapshot(`${name} ALL IN $${amount}`, evt.event_type, act);
            break;
        }
        actionOn = -1;
        break;
      }

      case "community_dealt": {
        const evtPhase = p.phase as string;
        community = (p.board as string[]) || community;
        phase = evtPhase || phase;
        resetBettingRound();
        const phaseLabel = evtPhase
          ? evtPhase.charAt(0).toUpperCase() + evtPhase.slice(1)
          : phase;
        snapshot(`${phaseLabel}`, evt.event_type);
        break;
      }

      case "showdown": {
        phase = "showdown";
        community = (p.board as string[]) || community;
        const sdPlayers = p.players as {
          seat: number;
          hole: string[];
          hand_desc: string;
        }[];
        if (sdPlayers) {
          for (const sp of sdPlayers) {
            if (players[sp.seat]) {
              players[sp.seat].hole = sp.hole;
            }
          }
        }
        snapshot("Showdown", evt.event_type);
        break;
      }

      case "pot_awarded": {
        const winners = p.winners as {
          seat: number;
          amount: number;
          player_id: string;
        }[];
        if (winners) {
          const winnerNames = winners
            .map((w) => {
              if (players[w.seat]) {
                players[w.seat].chips += w.amount;
              }
              return getPlayerName(w.player_id, w.seat);
            })
            .join(", ");
          const totalAmt = winners.reduce((s, w) => s + w.amount, 0);
          snapshot(`${winnerNames} wins $${totalAmt}`, evt.event_type);
          // Attach winner info to the last frame for animation
          const lastFrame = frames[frames.length - 1];
          if (lastFrame) {
            lastFrame.potAwardWinners = winners.map((w) => ({
              seat: w.seat,
              amount: w.amount,
            }));
          }
        }
        break;
      }

      case "player_eliminated": {
        const seat = p.seat as number;
        if (players[seat]) {
          players[seat].eliminated = true;
          const name = getPlayerName(
            players[seat].id,
            seat,
          );
          snapshot(`${name} eliminated`, evt.event_type);
        }
        break;
      }

      case "hand_end": {
        phase = "idle";
        // Update chips from authoritative hand_end data
        const endPlayers = p.players as { id: string; seat: number; chips: number }[];
        if (endPlayers) {
          for (const ep of endPlayers) {
            if (players[ep.seat]) {
              players[ep.seat].chips = ep.chips;
            }
          }
        }
        playerTotalBets.fill(0);
        resetBettingRound();
        snapshot(`Next Hand #${handNum + 1}`, evt.event_type);
        break;
      }

      case "game_over": {
        finished = true;
        phase = "showdown";
        snapshot("Game Over", evt.event_type);
        break;
      }
    }
  }

  return frames;
}
