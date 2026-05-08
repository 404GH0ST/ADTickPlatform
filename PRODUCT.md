## Design Context

### Users
This frontend serves both organizers and participants during an Attack-Defense CTF. Organizers use it to run the event, watch state changes, and recover quickly under pressure. Participants use it to monitor standings, manage their own services, and act fast during live play. Both groups are technical, but they are often time-constrained and operating under stress.

### Brand Personality
Calm, tactical, trustworthy.

The interface should feel steady under pressure, operational rather than theatrical, and credible enough for live event use.

### Aesthetic Direction
Use a restrained, security-oriented dashboard language inspired by Zellic and osec: clean structure, strong information hierarchy, low-noise surfaces, and practical typography. Support both light and dark themes. Favor operator efficiency over personality flourishes. Avoid AI-slop patterns, decorative gradients, inflated hero sections, and generic “premium dashboard” styling.

### Design Principles
1. Put the operational signal first. Important state, actions, and counts should be immediately scannable.
2. Keep copy direct and technical, but not backend-flavored. Users should understand the next action without decoding jargon.
3. Prefer calm surfaces and stable hierarchy over visual spectacle. The UI should support decision-making, not compete with it.
4. Reuse shared tokens and patterns. Custom visuals still need to belong to the same system.
5. Design for stress. Layouts, filters, and controls should remain readable when the page is dense or the operator is moving fast.
