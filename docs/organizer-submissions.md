# Organizer submission history

Open **Organizer → Submissions** (`/admin/submissions`) to inspect accepted,
rejected, and duplicate flag attempts. Filter by submitting team, victim team,
challenge, verdict, flag substring, or tick range. Flags can be expanded to show
the full submitted value. Use Refresh to load new attempts.

Delete acts on one history entry and requires confirmation:

- Rejected or duplicate attempt: remove that entry only. An existing accepted
  capture, its first blood, and its score remain intact.
- Accepted capture: remove its acceptance ledger entry and attack event, and
  recalculate score contributions. The same flag may be accepted again if valid.
- First-blood capture: also clear the challenge's first blood. Older normal
  captures remain normal; the next accepted capture earns the award. Deletion and
  acceptance use the same database lock, so concurrent captures produce one winner.

Deleted captures invalidate live attack slices, including large globe views.
A reaccepted flag receives a new event ID, allowing its live animation to run.
Deletion is recorded in the organizer audit log without the submitted flag.

Apply migration `0041_submission_history.sql` with the updated backend. Existing
accepted captures are imported, preserving their first-blood links and original
team names. Earlier rejected and duplicate attempts cannot be reconstructed;
those are logged only after this update. Flags rejected because the match is not
running are also recorded when they reach game-core. Malformed HTTP requests and
requests rejected by authentication or rate limiting do not create flag entries.

## See the live first-blood sequence

Keep `/attacks` or **Organizer → Attacks** (`/admin/attacks`) open on the live
feed, with the globe visible. The globe is present even before any captures.
Enable SFX and click a control once to allow browser audio. Maximize is available
while waiting for the first attack.

Submit a valid stolen flag for a challenge that has no first blood. The camera
frames both teams, the attacker charges, the strike travels to the defender,
and the defender marker breaks apart with a bass impact before reforming. The
camera then restores its previous view. The animation lasts about 6.5 seconds.
A continuous stereo score covers the camera, charge, strike, debris, and recovery.
After the camera settles, a closing banner names the capturing team and the
challenge while `lol-first-blood.mp3` plays. It stays visible for at least four
seconds, holds longer if the announcer is still playing, then fades out. The
result remains readable when sound is muted or unavailable. Long team and
challenge names wrap within the mobile banner.

Ordinary captures retain their existing sound. Multiple challenges' first bloods
play in order, waiting for both the closing banner and its announcer to finish.
Later captures use the normal animation; loading history, reloading the page,
and highlighting a tick do not replay a first blood. Reduced motion keeps the capture status without animated
zoom, shake, or fragments.

To repeat with real captures, delete that challenge's first-blood submission
from **Submissions**, then submit the next valid flag. Deletion changes scores;
use a test match when rehearsing. Do not reset the database just to test effects.

Automated browser checks use an isolated mock API, including an empty globe:

```sh
cd apps/web
bunx playwright test e2e/realtime-updates.spec.ts --grep 'first.blood'
```

## Organizer API

Both endpoints require the organizer's admin bearer token. Browser routes under
`/api/admin/submissions` additionally require an active organizer session and
same-origin mutation checks.

`GET /api/v2/admin/submissions`

Query parameters: `limit` (default 25, maximum 200), `offset`, `attacker`,
`victim`, `service`, `status` (`accepted`, `invalid`, `duplicate`), `flag`,
`tick_from`, `tick_to`. Text filters match literal substrings, ignoring case.
The response contains `items`, `limit`, `offset`, `total_count`, `has_prev`,
and `has_next`. Each item includes its numeric `id`, timestamp `submitted_at`,
`team_id`, `attacker`, `victim`, `challenge_id`, `service`, `flag`, `tick`,
`status`, `detail`, and current `first_blood` boolean.

`DELETE /api/v2/admin/submissions/{id}`

Returns `200` with `id`, `status`, `service`, and `first_blood_cleared`.
A missing or previously deleted ID returns `404`. A malformed ID returns `400`.
No automatic promotion of older captures occurs. Live score reads reflect the
remaining acceptance ledger; stored score snapshots are recomputed asynchronously.
