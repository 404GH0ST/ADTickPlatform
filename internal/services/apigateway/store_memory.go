package apigateway

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

type adminPlayerRecord struct {
	Player       adminPlayer
	PasswordHash string
	WireGuard    wireGuardPeerState
}

type serviceInstanceRecord struct {
	TeamID          int
	ChallengeID     int
	ChallengeName   string
	RuntimeKind     string
	RuntimeStatus   string
	DeploymentJobID int
	ContainerName   string
	StateVolume     string
	BaselineImage   string
	CheckerImage    string
	CheckerToken    string
	Endpoint        string
	SSHHost         string
	ServicePort     int
	UpdatedAt       string
}

type memoryStore struct {
	mu               sync.Mutex
	submitted        map[string]struct{}
	teamNames        map[int]string
	teams            map[int]*adminTeam
	players          map[int]*adminPlayerRecord
	challenges       map[int]*adminChallenge
	teamStates       map[int]map[int]*serviceState
	instances        map[int]map[int]*serviceInstanceRecord
	deployments      map[int]*adminDeploymentJob
	auditLogs        []adminAuditLog
	scoreboard       []scoreRow
	attackFeed       []attackEvent
	platformSettings adminPlatformSettings
	nextTeamID       int
	nextPlayerID     int
	nextChallengeID  int
	nextDeploymentID int
	nextAuditLogID   int
}

func NewMemoryStore(teamID int) Store {
	now := time.Now().UTC()
	store := &memoryStore{
		submitted: make(map[string]struct{}),
		teamNames: map[int]string{
			101: "Team Alpha",
			102: "Team Delta",
			103: "Team Sigma",
			104: "Team Orchid",
		},
		teams: map[int]*adminTeam{
			101: {ID: 101, Name: "Team Alpha", ContactEmail: "team-alpha@example.com", JoinKey: "TEAM-ALPHA-JOIN"},
			102: {ID: 102, Name: "Team Delta", ContactEmail: "team-delta@example.com", JoinKey: "TEAM-DELTA-JOIN"},
			103: {ID: 103, Name: "Team Sigma", ContactEmail: "team-sigma@example.com", JoinKey: "TEAM-SIGMA-JOIN"},
			104: {ID: 104, Name: "Team Orchid", ContactEmail: "team-orchid@example.com", JoinKey: "TEAM-ORCHID-JOIN"},
		},
		players: map[int]*adminPlayerRecord{
			1: mustNewAdminPlayerRecord(1, 101, "Team Alpha", "Alpha Captain", "alpha.captain@example.com", "captain", "alpha-secret", now.Add(-72*time.Hour)),
			2: mustNewAdminPlayerRecord(2, 102, "Team Delta", "Delta Captain", "delta.captain@example.com", "captain", "delta-secret", now.Add(-71*time.Hour)),
			3: mustNewAdminPlayerRecord(3, 103, "Team Sigma", "Sigma Captain", "sigma.captain@example.com", "captain", "sigma-secret", now.Add(-70*time.Hour)),
			4: mustNewAdminPlayerRecord(4, 104, "Team Orchid", "Orchid Captain", "orchid.captain@example.com", "captain", "orchid-secret", now.Add(-69*time.Hour)),
		},
		challenges: map[int]*adminChallenge{
			1: {ID: 1, Name: "banking", BaselineImage: "registry.local/banking:baseline", CheckerImage: "registry.local/banking-checker:latest", SourceBundlePath: "examples/sample-lfi-challenge", ServicePort: DefaultServicePort(1), ServiceSubnetOctet: DefaultServiceSubnetOctet(1), EgressEnabled: true, Published: true, CreatedAt: now.Add(-48 * time.Hour).Format(time.RFC3339)},
			2: {ID: 2, Name: "chat", BaselineImage: "registry.local/chat:baseline", CheckerImage: "registry.local/chat-checker:latest", SourceBundlePath: "examples/sample-rce-challenge", ServicePort: DefaultServicePort(2), ServiceSubnetOctet: DefaultServiceSubnetOctet(2), EgressEnabled: true, Published: true, CreatedAt: now.Add(-47 * time.Hour).Format(time.RFC3339)},
			3: {ID: 3, Name: "storage", BaselineImage: "registry.local/storage:baseline", CheckerImage: "registry.local/storage-checker:latest", ServicePort: DefaultServicePort(3), ServiceSubnetOctet: DefaultServiceSubnetOctet(3), EgressEnabled: true, Published: true, CreatedAt: now.Add(-46 * time.Hour).Format(time.RFC3339)},
		},
		teamStates:  make(map[int]map[int]*serviceState),
		instances:   make(map[int]map[int]*serviceInstanceRecord),
		deployments: make(map[int]*adminDeploymentJob),
		auditLogs:   []adminAuditLog{},
		scoreboard: []scoreRow{
			{Rank: 1, Team: "Team Alpha", Attack: 0, Defense: 0, SLA: 0, Total: 0, Delta: "0"},
			{Rank: 2, Team: "Team Delta", Attack: 0, Defense: 0, SLA: 0, Total: 0, Delta: "0"},
			{Rank: 3, Team: "Team Sigma", Attack: 0, Defense: 0, SLA: 0, Total: 0, Delta: "0"},
			{Rank: 4, Team: "Team Orchid", Attack: 0, Defense: 0, SLA: 0, Total: 0, Delta: "0"},
		},
		attackFeed:       []attackEvent{},
		platformSettings: adminPlatformSettings{FlagFormatPrefix: "PLAYIT", FlagFormatActive: "PLAYIT", MaxTeamMembers: 0, UpdatedBy: "system"},
		nextTeamID:       105,
		nextPlayerID:     5,
		nextChallengeID:  4,
		nextDeploymentID: 1,
		nextAuditLogID:   1,
	}

	for teamID := range store.teams {
		store.teamStates[teamID] = make(map[int]*serviceState)
		store.instances[teamID] = make(map[int]*serviceInstanceRecord)
		for challengeID, challenge := range store.challenges {
			if challenge.Published {
				store.teamStates[teamID][challengeID] = defaultServiceStateForConfig(challengeID, teamID, challenge.Name, challenge.ServicePort, challenge.ServiceSubnetOctet)
				store.instances[teamID][challengeID] = &serviceInstanceRecord{
					TeamID:        teamID,
					ChallengeID:   challengeID,
					ChallengeName: challenge.Name,
					RuntimeKind:   "docker",
					RuntimeStatus: "ready",
					ContainerName: serviceContainerName(challenge.Name, teamID),
					StateVolume:   serviceStateVolumeName(challenge.Name, teamID),
					BaselineImage: challenge.BaselineImage,
					CheckerImage:  challenge.CheckerImage,
					CheckerToken:  fmt.Sprintf("dev-checker-token-%d-%d", teamID, challengeID),
					Endpoint:      ServiceEndpointFor(challenge.ServiceSubnetOctet, challenge.ServicePort, teamID),
					SSHHost:       ServiceIP(challenge.ServiceSubnetOctet, teamID),
					ServicePort:   challenge.ServicePort,
					UpdatedAt:     now.Format(time.RFC3339),
				}
			}
		}
	}

	if _, ok := store.teams[teamID]; !ok {
		teamName := fmt.Sprintf("Team %d", teamID)
		store.teams[teamID] = &adminTeam{ID: teamID, Name: teamName, ContactEmail: fmt.Sprintf("team-%d@example.com", teamID), JoinKey: fmt.Sprintf("TEAM-%d-JOIN", teamID)}
		store.teamNames[teamID] = teamName
		store.teamStates[teamID] = make(map[int]*serviceState)
		store.instances[teamID] = make(map[int]*serviceInstanceRecord)
		for challengeID, challenge := range store.challenges {
			if challenge.Published {
				store.teamStates[teamID][challengeID] = defaultServiceStateForConfig(challengeID, teamID, challenge.Name, challenge.ServicePort, challenge.ServiceSubnetOctet)
				store.instances[teamID][challengeID] = &serviceInstanceRecord{
					TeamID:        teamID,
					ChallengeID:   challengeID,
					ChallengeName: challenge.Name,
					RuntimeKind:   "docker",
					RuntimeStatus: "ready",
					ContainerName: serviceContainerName(challenge.Name, teamID),
					StateVolume:   serviceStateVolumeName(challenge.Name, teamID),
					BaselineImage: challenge.BaselineImage,
					CheckerImage:  challenge.CheckerImage,
					CheckerToken:  fmt.Sprintf("dev-checker-token-%d-%d", teamID, challengeID),
					Endpoint:      ServiceEndpointFor(challenge.ServiceSubnetOctet, challenge.ServicePort, teamID),
					SSHHost:       ServiceIP(challenge.ServiceSubnetOctet, teamID),
					ServicePort:   challenge.ServicePort,
					UpdatedAt:     now.Format(time.RFC3339),
				}
			}
		}
		store.scoreboard = append(store.scoreboard, scoreRow{Rank: len(store.scoreboard) + 1, Team: teamName, Attack: 0, Defense: 0, SLA: 0, Total: 0, Delta: "new"})
	}

	return store
}

func (s *memoryStore) AuthenticatePlayer(_ context.Context, email, password string) (authenticatedPlayer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedEmail := strings.TrimSpace(strings.ToLower(email))

	for _, record := range s.players {
		if !strings.EqualFold(record.Player.Email, normalizedEmail) {
			continue
		}
		if !passwordMatches(record.PasswordHash, password) {
			return authenticatedPlayer{}, ErrInvalidCredentials
		}
		if passwordHashNeedsUpgrade(record.PasswordHash) {
			record.PasswordHash = mustHashPassword(password)
		}
		teamName := teamNameForID(s.teamNames, record.Player.TeamID)
		if record.Player.TeamID == 0 && !strings.EqualFold(record.Player.Role, "organizer") {
			teamName = ""
		}
		return authenticatedPlayer{
			PlayerID:    record.Player.ID,
			TeamID:      record.Player.TeamID,
			TeamName:    teamName,
			DisplayName: record.Player.DisplayName,
			Email:       record.Player.Email,
			Role:        record.Player.Role,
		}, nil
	}

	return authenticatedPlayer{}, ErrInvalidCredentials
}

func (s *memoryStore) RegisterPlayer(_ context.Context, input participantRegisterRequest, now time.Time) (authenticatedPlayer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	displayName := strings.TrimSpace(input.DisplayName)
	email := strings.TrimSpace(strings.ToLower(input.Email))
	if displayName == "" || email == "" || strings.TrimSpace(input.Password) == "" {
		return authenticatedPlayer{}, ErrDuplicateResource
	}
	for _, record := range s.players {
		if strings.EqualFold(record.Player.Email, email) {
			return authenticatedPlayer{}, ErrDuplicateResource
		}
	}

	id := s.nextPlayerID
	s.nextPlayerID++
	wireGuardPeer := wireguardPeerName(0, id)
	player := adminPlayer{
		ID:            id,
		TeamID:        0,
		TeamName:      "",
		DisplayName:   displayName,
		Email:         email,
		Role:          "member",
		WireGuardPeer: wireGuardPeer,
		CreatedAt:     now.UTC().Format(time.RFC3339),
	}
	s.players[id] = &adminPlayerRecord{Player: player, PasswordHash: mustHashPassword(input.Password)}
	return authenticatedPlayer{
		PlayerID:    id,
		TeamID:      0,
		TeamName:    "",
		DisplayName: displayName,
		Email:       email,
		Role:        "member",
	}, nil
}

func (s *memoryStore) ValidatePlayerSession(_ context.Context, playerID, teamID int, role string) (authenticatedPlayer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.players[playerID]
	if !ok {
		return authenticatedPlayer{}, ErrInvalidCredentials
	}
	if record.Player.TeamID != teamID {
		return authenticatedPlayer{}, ErrInvalidCredentials
	}
	if strings.TrimSpace(record.Player.Role) != strings.TrimSpace(role) {
		return authenticatedPlayer{}, ErrInvalidCredentials
	}
	teamName := teamNameForID(s.teamNames, record.Player.TeamID)
	if record.Player.TeamID == 0 && !strings.EqualFold(record.Player.Role, "organizer") {
		teamName = ""
	}
	return authenticatedPlayer{
		PlayerID:    record.Player.ID,
		TeamID:      record.Player.TeamID,
		TeamName:    teamName,
		DisplayName: record.Player.DisplayName,
		Email:       record.Player.Email,
		Role:        record.Player.Role,
	}, nil
}

func (s *memoryStore) ListChallenges(_ context.Context) ([]challenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]challenge, 0, len(s.challenges))
	for _, item := range s.challenges {
		if item.Published {
			result = append(result, challenge{
				ID:                item.ID,
				Name:              item.Name,
				HasSourceDownload: strings.TrimSpace(item.SourceBundlePath) != "",
			})
		}
	}
	slices.SortFunc(result, func(a, b challenge) int { return a.ID - b.ID })
	return result, nil
}

func (s *memoryStore) ListPublicServices(_ context.Context) (map[string]map[string][]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	services := make(map[string]map[string][]string)
	for teamID, challengeStates := range s.teamStates {
		for challengeID, state := range challengeStates {
			challengeMeta := s.challenges[challengeID]
			if challengeMeta == nil || !challengeMeta.Published {
				continue
			}
			challengeKey := fmt.Sprintf("%d", challengeID)
			if _, ok := services[challengeKey]; !ok {
				services[challengeKey] = make(map[string][]string)
			}
			services[challengeKey][fmt.Sprintf("%d", teamID)] = []string{state.Endpoint}
		}
	}
	return services, nil
}

func (s *memoryStore) ListScoreboard(_ context.Context) ([]scoreRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]scoreRow(nil), s.scoreboard...), nil
}

func (s *memoryStore) ListAttackFeed(_ context.Context) ([]attackEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]attackEvent(nil), s.attackFeed...), nil
}

func (s *memoryStore) ListTeamServices(_ context.Context, teamID int) ([]serviceState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	challengeStates, ok := s.teamStates[teamID]
	if !ok {
		return nil, nil
	}

	states := make([]serviceState, 0, len(challengeStates))
	for challengeID, state := range challengeStates {
		if challengeMeta := s.challenges[challengeID]; challengeMeta == nil || !challengeMeta.Published {
			continue
		}
		states = append(states, *cloneServiceState(state))
	}
	slices.SortFunc(states, func(a, b serviceState) int { return a.ChallengeID - b.ChallengeID })
	return states, nil
}

func (s *memoryStore) SubmitFlags(_ context.Context, teamID int, flags []string) ([]submissionVerdict, error) {
	return nil, ErrSubmissionUnavailable
}

func (s *memoryStore) ValidateServiceAction(_ context.Context, teamID, challengeID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.lookupTeamStateLocked(teamID, challengeID); err != nil {
		return err
	}
	challenge, ok := s.challenges[challengeID]
	if !ok || !challenge.Published {
		return ErrChallengeNotFound
	}
	instances, ok := s.instances[teamID]
	if !ok {
		return ErrChallengeNotFound
	}
	instance, ok := instances[challengeID]
	if !ok {
		return ErrChallengeNotFound
	}
	if strings.TrimSpace(instance.RuntimeStatus) != "ready" {
		return ErrServiceUnavailable
	}
	return nil
}

func (s *memoryStore) UnlockService(_ context.Context, teamID, challengeID int) (unlockData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.lookupTeamStateLocked(teamID, challengeID)
	if err != nil {
		return unlockData{}, err
	}
	challenge, ok := s.challenges[challengeID]
	if !ok || !challenge.Published {
		return unlockData{}, ErrChallengeNotFound
	}
	instance, ok := s.instances[teamID][challengeID]
	if !ok {
		return unlockData{}, ErrChallengeNotFound
	}
	if strings.TrimSpace(instance.RuntimeStatus) != "ready" {
		return unlockData{}, ErrServiceUnavailable
	}
	state.Unlocked = true
	state.Status = demoteStatus(state.Status)
	state.SSHHint = "unlock accepted; use SSH Access to view the team credential"
	state.LastEvent = "unlock granted via participant API"

	return unlockData{ChallengeID: challengeID, TeamID: teamID, Unlocked: true}, nil
}

func (s *memoryStore) CreateSSHSession(_ context.Context, teamID, challengeID int, _ time.Time) (sshSessionData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.lookupTeamStateLocked(teamID, challengeID)
	if err != nil {
		return sshSessionData{}, err
	}
	challenge, ok := s.challenges[challengeID]
	if !ok || !challenge.Published {
		return sshSessionData{}, ErrChallengeNotFound
	}
	instance, ok := s.instances[teamID][challengeID]
	if !ok {
		return sshSessionData{}, ErrChallengeNotFound
	}
	if strings.TrimSpace(instance.RuntimeStatus) != "ready" {
		return sshSessionData{}, ErrServiceUnavailable
	}
	if !state.Unlocked {
		return sshSessionData{}, ErrServiceLocked
	}

	host, _ := ParseEndpoint(state.Endpoint)
	connectionHint := fmt.Sprintf("ssh %s@%s", sshUsername(), host)
	state.SSHHint = connectionHint
	state.LastEvent = "team ssh credential retrieved"

	return sshSessionData{ChallengeID: challengeID, Host: host, Port: 22, Username: sshUsername(), PasswordMode: "stable", ConnectionHint: connectionHint}, nil
}

func (s *memoryStore) MarkSSHSessionApplyFailure(_ context.Context, teamID, challengeID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.lookupTeamStateLocked(teamID, challengeID)
	if err != nil {
		return err
	}
	state.SSHHint = "credential apply failed; open SSH Access to retry applying the team credential"
	state.LastEvent = "ssh credential apply failed"
	return nil
}

func (s *memoryStore) PrepareFactoryResetService(_ context.Context, teamID, challengeID int) (resetData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.lookupTeamStateLocked(teamID, challengeID)
	if err != nil {
		return resetData{}, err
	}
	challenge, ok := s.challenges[challengeID]
	if !ok || !challenge.Published {
		return resetData{}, ErrChallengeNotFound
	}
	instance, ok := s.instances[teamID][challengeID]
	if !ok {
		return resetData{}, ErrChallengeNotFound
	}
	if strings.TrimSpace(instance.RuntimeStatus) != "ready" {
		return resetData{}, ErrServiceUnavailable
	}
	checkerToken, err := newCheckerToken()
	if err != nil {
		return resetData{}, err
	}
	instance.CheckerToken = checkerToken
	state.Status = "resetting"
	state.Checker = "warning"
	state.LastEvent = "factory reset started via participant API"
	state.ResetCooldown = "resetting"

	return resetData{ChallengeID: challengeID, TeamID: teamID, Action: "factory_reset", UnlockPreserved: state.Unlocked}, nil
}

func (s *memoryStore) CompleteFactoryResetService(_ context.Context, teamID, challengeID int) (resetData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.lookupTeamStateLocked(teamID, challengeID)
	if err != nil {
		return resetData{}, err
	}
	state.Status = "warming"
	state.Checker = "warning"
	state.LastEvent = "factory reset completed; waiting for checker verification"
	state.ResetCooldown = "cooldown: 90s"
	if state.Unlocked {
		state.SSHHint = "unlock preserved; open SSH Access to reapply the team credential"
	}
	return resetData{ChallengeID: challengeID, TeamID: teamID, Action: "factory_reset", UnlockPreserved: state.Unlocked}, nil
}

func (s *memoryStore) MarkFactoryResetFailure(_ context.Context, teamID, challengeID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.lookupTeamStateLocked(teamID, challengeID)
	if err != nil {
		return err
	}
	state.Status = "degraded"
	state.Checker = "warning"
	state.LastEvent = "factory reset runtime failed"
	state.ResetCooldown = "ready"
	return nil
}

func (s *memoryStore) RestartService(_ context.Context, teamID, challengeID int) (resetData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.lookupTeamStateLocked(teamID, challengeID)
	if err != nil {
		return resetData{}, err
	}
	challenge, ok := s.challenges[challengeID]
	if !ok || !challenge.Published {
		return resetData{}, ErrChallengeNotFound
	}
	instance, ok := s.instances[teamID][challengeID]
	if !ok {
		return resetData{}, ErrChallengeNotFound
	}
	if strings.TrimSpace(instance.RuntimeStatus) != "ready" {
		return resetData{}, ErrServiceUnavailable
	}
	state.Status = "warming"
	state.Checker = "warning"
	state.LastEvent = "service restart completed; waiting for checker verification"
	state.ResetCooldown = "cooldown: 30s"

	return resetData{ChallengeID: challengeID, TeamID: teamID, Action: "restart"}, nil
}

func (s *memoryStore) ListAdminTeams(_ context.Context) ([]adminTeam, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	teams := make([]adminTeam, 0, len(s.teams))
	for _, team := range s.teams {
		clone := *team
		clone.PlayerCount = s.playerCountForTeamLocked(team.ID)
		clone.DeployedChallenges = len(s.teamStates[team.ID])
		teams = append(teams, clone)
	}
	slices.SortFunc(teams, func(a, b adminTeam) int { return a.ID - b.ID })
	return teams, nil
}

func (s *memoryStore) CreateAdminTeam(_ context.Context, input adminCreateTeamRequest) (adminTeam, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := strings.TrimSpace(input.Name)
	contactEmail := strings.TrimSpace(strings.ToLower(input.ContactEmail))
	if name == "" || contactEmail == "" {
		return adminTeam{}, ErrDuplicateResource
	}
	for _, team := range s.teams {
		if strings.EqualFold(team.Name, name) || strings.EqualFold(team.ContactEmail, contactEmail) {
			return adminTeam{}, ErrDuplicateResource
		}
	}

	id := s.nextTeamID
	s.nextTeamID++
	joinKey, err := NewTeamJoinKey()
	if err != nil {
		return adminTeam{}, err
	}
	team := &adminTeam{ID: id, Name: name, ContactEmail: contactEmail, JoinKey: joinKey}
	s.teams[id] = team
	s.teamNames[id] = name
	s.teamStates[id] = make(map[int]*serviceState)
	s.instances[id] = make(map[int]*serviceInstanceRecord)
	for challengeID, challenge := range s.challenges {
		if challenge.Published {
			state := defaultServiceStateForConfig(challengeID, id, challenge.Name, challenge.ServicePort, challenge.ServiceSubnetOctet)
			state.Status = "provisioning"
			state.Checker = "pending"
			state.SSHHint = "deployment queued; wait for controller reconciliation before unlock"
			state.LastEvent = "baseline deployment queued by team join"
			state.ResetCooldown = "deploying"
			s.teamStates[id][challengeID] = state

			checkerToken, err := newCheckerToken()
			if err != nil {
				return adminTeam{}, err
			}
			s.instances[id][challengeID] = &serviceInstanceRecord{
				TeamID:        id,
				ChallengeID:   challengeID,
				ChallengeName: challenge.Name,
				RuntimeKind:   "docker",
				RuntimeStatus: "queued",
				ContainerName: serviceContainerName(challenge.Name, id),
				StateVolume:   serviceStateVolumeName(challenge.Name, id),
				BaselineImage: challenge.BaselineImage,
				CheckerImage:  challenge.CheckerImage,
				CheckerToken:  checkerToken,
				Endpoint:      ServiceEndpointFor(challenge.ServiceSubnetOctet, challenge.ServicePort, id),
				SSHHost:       ServiceIP(challenge.ServiceSubnetOctet, id),
				ServicePort:   challenge.ServicePort,
				UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
			}
		}
	}
	s.scoreboard = append(s.scoreboard, scoreRow{Rank: len(s.scoreboard) + 1, Team: name, Attack: 0, Defense: 0, SLA: 0, Total: 0, Delta: "new"})

	return adminTeam{ID: id, Name: name, ContactEmail: contactEmail, PlayerCount: 0, DeployedChallenges: len(s.teamStates[id])}, nil
}

func (s *memoryStore) ListAdminPlayers(_ context.Context) ([]adminPlayer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	players := make([]adminPlayer, 0, len(s.players))
	for _, record := range s.players {
		player := record.Player
		if player.TeamID > 0 {
			player.TeamName = teamNameForID(s.teamNames, player.TeamID)
		} else if strings.EqualFold(strings.TrimSpace(player.Role), "organizer") {
			player.TeamName = "Organizer"
		} else {
			player.TeamName = ""
		}
		if player.TeamID > 0 || strings.EqualFold(strings.TrimSpace(player.Role), "organizer") {
			player.WireGuardAddress = record.WireGuard.Address
			player.WireGuardStatus = record.WireGuard.Status
			player.WireGuardIssuedAt = record.WireGuard.IssuedAt
			player.WireGuardRevokedAt = record.WireGuard.RevokedAt
		} else {
			player.WireGuardPeer = ""
		}
		players = append(players, player)
	}
	slices.SortFunc(players, func(a, b adminPlayer) int { return a.ID - b.ID })
	return players, nil
}

func (s *memoryStore) CreateAdminPlayer(_ context.Context, input adminCreatePlayerRequest, now time.Time) (adminPlayer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.TeamID != 0 {
		if _, ok := s.teams[input.TeamID]; !ok {
			return adminPlayer{}, ErrTeamNotFound
		}
		if s.teamMemberLimitReachedLocked(input.TeamID) {
			return adminPlayer{}, ErrTeamMemberLimit
		}
	}
	email := strings.TrimSpace(strings.ToLower(input.Email))
	for _, record := range s.players {
		if strings.EqualFold(record.Player.Email, email) {
			return adminPlayer{}, ErrDuplicateResource
		}
	}

	id := s.nextPlayerID
	s.nextPlayerID++
	wireGuardPeer := wireguardPeerName(input.TeamID, id)

	role := normalizedRole(input.Role)
	teamName := ""
	if input.TeamID != 0 {
		teamName = teamNameForID(s.teamNames, input.TeamID)
	} else if role == "organizer" {
		teamName = "Organizer"
	}

	player := adminPlayer{
		ID:            id,
		TeamID:        input.TeamID,
		TeamName:      teamName,
		DisplayName:   strings.TrimSpace(input.DisplayName),
		Email:         email,
		Role:          role,
		WireGuardPeer: wireGuardPeer,
		CreatedAt:     now.UTC().Format(time.RFC3339),
	}
	record := &adminPlayerRecord{Player: player, PasswordHash: mustHashPassword(input.Password)}
	if player.TeamID > 0 || player.Role == "organizer" {
		wireGuardState, err := newWireGuardPeerState(id, input.TeamID, teamName, strings.TrimSpace(input.DisplayName), wireGuardPeer, now)
		if err != nil {
			return adminPlayer{}, err
		}
		player.WireGuardAddress = wireGuardState.Address
		player.WireGuardStatus = wireGuardState.Status
		player.WireGuardIssuedAt = wireGuardState.IssuedAt
		player.WireGuardRevokedAt = wireGuardState.RevokedAt
		record.Player = player
		record.WireGuard = wireGuardState
	}
	s.players[id] = record
	return player, nil
}

func (s *memoryStore) JoinExistingPlayerTeam(_ context.Context, playerID int, teamKey string, now time.Time) (authenticatedPlayer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.players[playerID]
	if !ok {
		return authenticatedPlayer{}, ErrInvalidCredentials
	}
	if strings.EqualFold(strings.TrimSpace(record.Player.Role), "organizer") || record.Player.TeamID > 0 {
		return authenticatedPlayer{}, ErrDuplicateResource
	}

	var team *adminTeam
	for _, candidate := range s.teams {
		if strings.EqualFold(candidate.JoinKey, strings.TrimSpace(teamKey)) {
			team = candidate
			break
		}
	}
	if team == nil {
		return authenticatedPlayer{}, ErrInvalidCredentials
	}
	if s.teamMemberLimitReachedLocked(team.ID) {
		return authenticatedPlayer{}, ErrTeamMemberLimit
	}

	wireGuardPeer := wireguardPeerName(team.ID, playerID)
	wireGuardState, err := newWireGuardPeerState(playerID, team.ID, team.Name, record.Player.DisplayName, wireGuardPeer, now)
	if err != nil {
		return authenticatedPlayer{}, err
	}
	record.Player.TeamID = team.ID
	record.Player.TeamName = team.Name
	record.Player.WireGuardPeer = wireGuardPeer
	record.Player.WireGuardAddress = wireGuardState.Address
	record.Player.WireGuardStatus = wireGuardState.Status
	record.Player.WireGuardIssuedAt = wireGuardState.IssuedAt
	record.Player.WireGuardRevokedAt = wireGuardState.RevokedAt
	record.WireGuard = wireGuardState

	return authenticatedPlayer{
		PlayerID:    playerID,
		TeamID:      team.ID,
		TeamName:    team.Name,
		DisplayName: record.Player.DisplayName,
		Email:       record.Player.Email,
		Role:        record.Player.Role,
	}, nil
}

func (s *memoryStore) teamMemberLimitReachedLocked(teamID int) bool {
	limit := s.platformSettings.MaxTeamMembers
	if teamID <= 0 || limit <= 0 {
		return false
	}
	count := 0
	for _, record := range s.players {
		if record.Player.TeamID == teamID {
			count++
		}
	}
	return count >= limit
}

func (s *memoryStore) GetAdminPlayerWireGuardConfig(_ context.Context, playerID int) (adminWireGuardPeer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.players[playerID]
	if !ok {
		return adminWireGuardPeer{}, ErrPlayerNotFound
	}
	if record.Player.TeamID <= 0 && !strings.EqualFold(strings.TrimSpace(record.Player.Role), "organizer") {
		return adminWireGuardPeer{}, ErrPlayerNotFound
	}
	record.WireGuard.TeamName = teamNameForID(s.teamNames, record.Player.TeamID)
	if strings.EqualFold(strings.TrimSpace(record.Player.Role), "organizer") && record.Player.TeamID == 0 {
		record.WireGuard.TeamName = "Organizer"
	}
	refreshedState, changed, err := refreshWireGuardPeerState(record.WireGuard)
	if err != nil {
		return adminWireGuardPeer{}, err
	}
	if changed {
		record.WireGuard = refreshedState
	}
	return wireGuardAdminView(record.WireGuard, record.Player.Email), nil
}

func (s *memoryStore) RotateAdminPlayerWireGuardConfig(_ context.Context, playerID int, now time.Time) (adminWireGuardPeer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.players[playerID]
	if !ok {
		return adminWireGuardPeer{}, ErrPlayerNotFound
	}
	if record.Player.TeamID <= 0 && !strings.EqualFold(strings.TrimSpace(record.Player.Role), "organizer") {
		return adminWireGuardPeer{}, ErrPlayerNotFound
	}
	teamName := teamNameForID(s.teamNames, record.Player.TeamID)
	if strings.EqualFold(strings.TrimSpace(record.Player.Role), "organizer") && record.Player.TeamID == 0 {
		teamName = "Organizer"
	}
	wireGuardState, err := newWireGuardPeerState(record.Player.ID, record.Player.TeamID, teamName, record.Player.DisplayName, record.Player.WireGuardPeer, now)
	if err != nil {
		return adminWireGuardPeer{}, err
	}
	record.WireGuard = wireGuardState
	record.Player.WireGuardAddress = wireGuardState.Address
	record.Player.WireGuardStatus = wireGuardState.Status
	record.Player.WireGuardIssuedAt = wireGuardState.IssuedAt
	record.Player.WireGuardRevokedAt = ""
	return wireGuardAdminView(record.WireGuard, record.Player.Email), nil
}

func (s *memoryStore) RevokeAdminPlayerWireGuardConfig(_ context.Context, playerID int, now time.Time) (adminWireGuardPeer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.players[playerID]
	if !ok {
		return adminWireGuardPeer{}, ErrPlayerNotFound
	}
	if record.Player.TeamID <= 0 && !strings.EqualFold(strings.TrimSpace(record.Player.Role), "organizer") {
		return adminWireGuardPeer{}, ErrPlayerNotFound
	}
	record.WireGuard.Status = "revoked"
	record.WireGuard.RevokedAt = now.UTC().Format(time.RFC3339)
	record.Player.WireGuardStatus = record.WireGuard.Status
	record.Player.WireGuardRevokedAt = record.WireGuard.RevokedAt
	return wireGuardAdminView(record.WireGuard, record.Player.Email), nil
}

func (s *memoryStore) ListWireGuardGatewayPeers(_ context.Context) ([]WireGuardGatewayPeer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	playerIDs := make([]int, 0, len(s.players))
	for playerID := range s.players {
		playerIDs = append(playerIDs, playerID)
	}
	slices.Sort(playerIDs)

	peers := make([]WireGuardGatewayPeer, 0, len(playerIDs))
	for _, playerID := range playerIDs {
		record := s.players[playerID]
		if record.Player.TeamID <= 0 && !strings.EqualFold(strings.TrimSpace(record.Player.Role), "organizer") {
			continue
		}
		record.WireGuard.TeamName = teamNameForID(s.teamNames, record.Player.TeamID)
		if strings.EqualFold(strings.TrimSpace(record.Player.Role), "organizer") && record.Player.TeamID == 0 {
			record.WireGuard.TeamName = "Organizer"
		}
		peers = append(peers, WireGuardGatewayPeer{
			PlayerID:        record.Player.ID,
			TeamID:          record.Player.TeamID,
			TeamName:        record.WireGuard.TeamName,
			DisplayName:     record.Player.DisplayName,
			WireGuardPeer:   record.Player.WireGuardPeer,
			Address:         record.WireGuard.Address,
			Status:          record.WireGuard.Status,
			ClientPublicKey: record.WireGuard.ClientPublicKey,
			PresharedKey:    record.WireGuard.PresharedKey,
		})
	}
	return peers, nil
}

func (s *memoryStore) IsMatchPaused(_ context.Context) (bool, error) {
	return false, nil
}

func (s *memoryStore) ListAdminChallenges(_ context.Context) ([]adminChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	challenges := make([]adminChallenge, 0, len(s.challenges))
	totalTeams := len(s.teams)
	for _, challenge := range s.challenges {
		clone := *challenge
		clone.TotalTeams = totalTeams
		clone.DeployedTeams = s.deployedTeamsForChallengeLocked(challenge.ID)
		clone.ReadyTeams = s.readyTeamsForChallengeLocked(challenge.ID)
		clone.QueuedTeams = s.queuedTeamsForChallengeLocked(challenge.ID)
		clone.RuntimeStatus = challengeRuntimeStatus(clone.Published, clone.TotalTeams, clone.ReadyTeams, clone.QueuedTeams)
		challenges = append(challenges, clone)
	}
	slices.SortFunc(challenges, func(a, b adminChallenge) int { return a.ID - b.ID })
	return challenges, nil
}

func (s *memoryStore) CreateAdminChallenge(_ context.Context, input adminCreateChallengeRequest, now time.Time) (adminChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return adminChallenge{}, ErrDuplicateResource
	}
	for _, challenge := range s.challenges {
		if strings.EqualFold(challenge.Name, name) {
			return adminChallenge{}, ErrDuplicateResource
		}
	}

	baselineImage := strings.TrimSpace(input.BaselineImage)
	checkerImage := strings.TrimSpace(input.CheckerImage)
	if baselineImage == "" || checkerImage == "" {
		defaultBaseline, defaultChecker := defaultChallengeImages(name)
		if baselineImage == "" {
			baselineImage = defaultBaseline
		}
		if checkerImage == "" {
			checkerImage = defaultChecker
		}
	}

	id := s.nextChallengeID
	s.nextChallengeID++
	servicePort := input.ServicePort
	if servicePort <= 0 {
		servicePort = DefaultServicePort(id)
	}
	serviceSubnetOctet := input.ServiceSubnetOctet
	if serviceSubnetOctet <= 0 {
		serviceSubnetOctet = DefaultServiceSubnetOctet(id)
	}
	if err := validateChallengeRuntimeConfig(servicePort, serviceSubnetOctet); err != nil {
		return adminChallenge{}, err
	}
	for existingID, challenge := range s.challenges {
		if existingID == id {
			continue
		}
		if challenge.ServiceSubnetOctet == serviceSubnetOctet {
			return adminChallenge{}, fmt.Errorf("%w: service_subnet_octet %d is already assigned to challenge %d", ErrInvalidRuntimeConfig, serviceSubnetOctet, challenge.ID)
		}
	}
	egressEnabled := true
	if input.EgressEnabled != nil {
		egressEnabled = *input.EgressEnabled
	}
	challenge := adminChallenge{
		ID:                 id,
		Name:               name,
		BaselineImage:      baselineImage,
		CheckerImage:       checkerImage,
		SourceBundlePath:   sanitizeSourceBundlePath(input.SourceBundlePath),
		ServicePort:        servicePort,
		ServiceSubnetOctet: serviceSubnetOctet,
		EgressEnabled:      egressEnabled,
		Published:          false,
		TotalTeams:         len(s.teams),
		RuntimeStatus:      "draft",
		CreatedAt:          now.UTC().Format(time.RFC3339),
	}
	s.challenges[id] = &challenge
	return challenge, nil
}

func (s *memoryStore) DeployAdminChallenge(_ context.Context, challengeID int) (adminDeployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	challenge, ok := s.challenges[challengeID]
	if !ok {
		return adminDeployment{}, ErrChallengeNotFound
	}

	jobID := 0
	queuedCount := 0
	readyCount := 0
	createdAt := time.Now().UTC().Format(time.RFC3339)
	for teamID := range s.teams {
		if _, ok := s.teamStates[teamID]; !ok {
			s.teamStates[teamID] = make(map[int]*serviceState)
		}
		if _, ok := s.instances[teamID]; !ok {
			s.instances[teamID] = make(map[int]*serviceInstanceRecord)
		}
		if jobID == 0 {
			jobID = s.nextDeploymentID
			s.nextDeploymentID++
		}
		checkerToken, err := newCheckerToken()
		if err != nil {
			return adminDeployment{}, err
		}
		instance := &serviceInstanceRecord{
			TeamID:          teamID,
			ChallengeID:     challengeID,
			ChallengeName:   challenge.Name,
			RuntimeKind:     "docker",
			RuntimeStatus:   "queued",
			DeploymentJobID: jobID,
			ContainerName:   serviceContainerName(challenge.Name, teamID),
			StateVolume:     serviceStateVolumeName(challenge.Name, teamID),
			BaselineImage:   challenge.BaselineImage,
			CheckerImage:    challenge.CheckerImage,
			CheckerToken:    checkerToken,
			Endpoint:        ServiceEndpointFor(challenge.ServiceSubnetOctet, challenge.ServicePort, teamID),
			SSHHost:         ServiceIP(challenge.ServiceSubnetOctet, teamID),
			ServicePort:     challenge.ServicePort,
			UpdatedAt:       createdAt,
		}
		s.instances[teamID][challengeID] = instance
		s.teamStates[teamID][challengeID] = &serviceState{
			ChallengeID:   challengeID,
			TeamID:        teamID,
			Name:          challenge.Name,
			Endpoint:      instance.Endpoint,
			Status:        "provisioning",
			Checker:       "pending",
			Unlocked:      false,
			SSHHint:       "deployment queued; wait for controller reconciliation before unlock",
			LastEvent:     "baseline deployment queued by organizer",
			ResetCooldown: "deploying",
		}
		queuedCount++
	}
	challenge.Published = true
	challenge.TotalTeams = len(s.teams)
	challenge.DeployedTeams = s.deployedTeamsForChallengeLocked(challengeID)
	challenge.ReadyTeams = readyCount
	challenge.QueuedTeams = queuedCount
	challenge.RuntimeStatus = challengeRuntimeStatus(true, challenge.TotalTeams, challenge.ReadyTeams, challenge.QueuedTeams)

	for _, deployment := range s.deployments {
		if deployment.ChallengeID != challengeID || deployment.ID == jobID {
			continue
		}
		if deployment.Status != "queued" && deployment.Status != "running" {
			continue
		}
		deployment.Status = "superseded"
		deployment.QueuedTeamCount = 0
		deployment.CompletedAt = createdAt
	}

	status := "completed"
	if queuedCount > 0 {
		status = "queued"
		s.deployments[jobID] = &adminDeploymentJob{
			ID:              jobID,
			ChallengeID:     challengeID,
			ChallengeName:   challenge.Name,
			Status:          status,
			TargetTeamCount: len(s.teams),
			QueuedTeamCount: queuedCount,
			ReadyTeamCount:  readyCount,
			CreatedAt:       createdAt,
		}
	}

	return adminDeployment{
		JobID:             jobID,
		ChallengeID:       challengeID,
		ChallengeName:     challenge.Name,
		Status:            status,
		Published:         true,
		DeployedTeamCount: queuedCount,
		TotalTeamCount:    len(s.teams),
		QueuedTeamCount:   queuedCount,
		ReadyTeamCount:    readyCount,
		CreatedAt:         createdAt,
	}, nil
}

func (s *memoryStore) ListAdminDeployments(_ context.Context) ([]adminDeploymentJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	deployments := make([]adminDeploymentJob, 0, len(s.deployments))
	for _, deployment := range s.deployments {
		deployments = append(deployments, *deployment)
	}
	slices.SortFunc(deployments, func(a, b adminDeploymentJob) int { return b.ID - a.ID })
	return deployments, nil
}

func (s *memoryStore) DeleteAdminDeployment(_ context.Context, deploymentID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.deployments[deploymentID]; !ok {
		return ErrDeploymentNotFound
	}

	for _, challengeInstances := range s.instances {
		for _, instance := range challengeInstances {
			if instance.DeploymentJobID == deploymentID && instance.RuntimeStatus == "queued" {
				return ErrDeploymentActive
			}
		}
	}

	delete(s.deployments, deploymentID)
	for _, challengeInstances := range s.instances {
		for _, instance := range challengeInstances {
			if instance.DeploymentJobID == deploymentID {
				instance.DeploymentJobID = 0
			}
		}
	}
	return nil
}

func (s *memoryStore) ListAdminAuditLogs(_ context.Context, query adminAuditLogQuery) (adminAuditLogPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]adminAuditLog, 0, len(s.auditLogs))
	for _, logEntry := range s.auditLogs {
		if query.ActorType != "" && !strings.EqualFold(logEntry.ActorType, query.ActorType) {
			continue
		}
		if query.Action != "" && !strings.Contains(strings.ToLower(logEntry.Action), strings.ToLower(query.Action)) {
			continue
		}
		if query.TargetType != "" && !strings.EqualFold(logEntry.TargetType, query.TargetType) {
			continue
		}
		if query.Status != "" && !strings.EqualFold(logEntry.Status, query.Status) {
			continue
		}
		filtered = append(filtered, logEntry)
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 25
	}
	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	page := adminAuditLogPage{
		Limit:      limit,
		Offset:     offset,
		TotalCount: len(filtered),
		HasPrev:    offset > 0,
	}
	if offset >= len(filtered) {
		page.Items = []adminAuditLog{}
		return page, nil
	}
	if limit > len(filtered)-offset {
		limit = len(filtered) - offset
	}
	page.HasNext = offset+limit < len(filtered)
	page.Items = append([]adminAuditLog(nil), filtered[offset:offset+limit]...)
	return page, nil
}

func (s *memoryStore) AppendAdminAuditLog(_ context.Context, entry adminAuditLogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadata := strings.TrimSpace(entry.Metadata)
	if metadata == "" {
		metadata = "{}"
	}

	s.auditLogs = append([]adminAuditLog{{
		ID:         s.nextAuditLogID,
		ActorType:  strings.TrimSpace(entry.ActorType),
		Actor:      strings.TrimSpace(entry.Actor),
		Action:     strings.TrimSpace(entry.Action),
		TargetType: strings.TrimSpace(entry.TargetType),
		Target:     strings.TrimSpace(entry.Target),
		Status:     strings.TrimSpace(entry.Status),
		Message:    strings.TrimSpace(entry.Message),
		Metadata:   metadata,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}}, s.auditLogs...)
	s.nextAuditLogID++
	return nil
}

func (s *memoryStore) GetPlatformSettings(_ context.Context) (adminPlatformSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.platformSettings, nil
}

func (s *memoryStore) UpdatePlatformSettings(_ context.Context, input adminUpdatePlatformSettingsRequest, actor string, now time.Time) (adminPlatformSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prefix := strings.TrimSpace(input.FlagFormatPrefix)
	if prefix == "" {
		return adminPlatformSettings{}, fmt.Errorf("flag_format_prefix must not be empty")
	}
	if input.MaxTeamMembers != nil && *input.MaxTeamMembers < 0 {
		return adminPlatformSettings{}, fmt.Errorf("max_team_members must not be negative")
	}
	s.platformSettings.FlagFormatPrefix = prefix
	if input.MaxTeamMembers != nil {
		s.platformSettings.MaxTeamMembers = *input.MaxTeamMembers
	}
	s.platformSettings.UpdatedAt = now.UTC().Format(time.RFC3339)
	s.platformSettings.UpdatedBy = strings.TrimSpace(actor)
	return s.platformSettings, nil
}

func (s *memoryStore) SetActiveFlagFormat(_ context.Context, format string, now time.Time) (adminPlatformSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	active := strings.TrimSpace(format)
	if active == "" {
		active = s.platformSettings.FlagFormatPrefix
	}
	s.platformSettings.FlagFormatActive = active
	s.platformSettings.UpdatedAt = now.UTC().Format(time.RFC3339)
	return s.platformSettings, nil
}

func (s *memoryStore) ListControllerRuntimeTasks(_ context.Context) ([]ControllerRuntimeTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks := make([]ControllerRuntimeTask, 0)
	for teamID, challengeInstances := range s.instances {
		for challengeID, instance := range challengeInstances {
			if instance.RuntimeStatus != "queued" {
				continue
			}
			tasks = append(tasks, ControllerRuntimeTask{
				DeploymentJobID: instance.DeploymentJobID,
				TeamID:          teamID,
				ChallengeID:     challengeID,
				ChallengeName:   instance.ChallengeName,
				RuntimeKind:     instance.RuntimeKind,
				ContainerName:   instance.ContainerName,
				StateVolume:     instance.StateVolume,
				BaselineImage:   instance.BaselineImage,
				CheckerToken:    instance.CheckerToken,
				Endpoint:        instance.Endpoint,
				SSHHost:         instance.SSHHost,
				ServicePort:     instance.ServicePort,
			})
		}
	}
	slices.SortFunc(tasks, func(a, b ControllerRuntimeTask) int {
		if a.DeploymentJobID != b.DeploymentJobID {
			return a.DeploymentJobID - b.DeploymentJobID
		}
		if a.ChallengeID != b.ChallengeID {
			return a.ChallengeID - b.ChallengeID
		}
		return a.TeamID - b.TeamID
	})
	return tasks, nil
}

func (s *memoryStore) GetControllerRuntimeTask(_ context.Context, teamID, challengeID int) (ControllerRuntimeTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	challengeInstances, ok := s.instances[teamID]
	if !ok {
		return ControllerRuntimeTask{}, ErrChallengeNotFound
	}
	instance, ok := challengeInstances[challengeID]
	if !ok {
		return ControllerRuntimeTask{}, ErrChallengeNotFound
	}

	return ControllerRuntimeTask{
		DeploymentJobID: instance.DeploymentJobID,
		TeamID:          teamID,
		ChallengeID:     challengeID,
		ChallengeName:   instance.ChallengeName,
		RuntimeKind:     instance.RuntimeKind,
		ContainerName:   instance.ContainerName,
		StateVolume:     instance.StateVolume,
		BaselineImage:   instance.BaselineImage,
		CheckerToken:    instance.CheckerToken,
		Endpoint:        instance.Endpoint,
		SSHHost:         instance.SSHHost,
		ServicePort:     instance.ServicePort,
	}, nil
}

func (s *memoryStore) ListControllerServiceAccessPolicies(_ context.Context) ([]ControllerServiceAccessPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	policies := make([]ControllerServiceAccessPolicy, 0)
	for teamID, challengeStates := range s.teamStates {
		for challengeID, state := range challengeStates {
			policies = append(policies, s.buildAccessPolicyLocked(teamID, challengeID, state))
		}
	}
	slices.SortFunc(policies, func(a, b ControllerServiceAccessPolicy) int {
		if a.ChallengeID != b.ChallengeID {
			return a.ChallengeID - b.ChallengeID
		}
		return a.TeamID - b.TeamID
	})
	return policies, nil
}

func (s *memoryStore) GetControllerServiceAccessPolicy(_ context.Context, teamID, challengeID int) (ControllerServiceAccessPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	challengeStates, ok := s.teamStates[teamID]
	if !ok {
		return ControllerServiceAccessPolicy{}, ErrChallengeNotFound
	}
	state, ok := challengeStates[challengeID]
	if !ok {
		return ControllerServiceAccessPolicy{}, ErrChallengeNotFound
	}
	return s.buildAccessPolicyLocked(teamID, challengeID, state), nil
}

func (s *memoryStore) buildAccessPolicyLocked(teamID, challengeID int, state *serviceState) ControllerServiceAccessPolicy {
	allowedPeers := make([]string, 0)
	serviceIP, servicePort := ParseEndpoint(state.Endpoint)

	for _, player := range s.players {
		if strings.EqualFold(strings.TrimSpace(player.WireGuard.Status), "revoked") {
			continue
		}
		// Allow team members only if unlocked
		if player.Player.TeamID == teamID && state.Unlocked {
			allowedPeers = append(allowedPeers, player.WireGuard.Address)
			continue
		}
		// Always allow organizers
		if player.Player.Role == "organizer" {
			allowedPeers = append(allowedPeers, player.WireGuard.Address)
		}
	}
	slices.Sort(allowedPeers)
	allowedPeers = slices.Compact(allowedPeers)

	egressEnabled := true
	if challenge, ok := s.challenges[challengeID]; ok && challenge != nil {
		egressEnabled = challenge.EgressEnabled
	}

	return ControllerServiceAccessPolicy{
		TeamID:               teamID,
		TeamName:             teamNameForID(s.teamNames, teamID),
		ChallengeID:          challengeID,
		ChallengeName:        state.Name,
		ServiceIP:            serviceIP,
		ServicePort:          servicePort,
		SSHPort:              22,
		SSHUnlocked:          state.Unlocked,
		AllowedPeerAddresses: allowedPeers,
		EgressEnabled:        egressEnabled,
	}
}

func (s *memoryStore) ReconcileAdminDeployments(_ context.Context, now time.Time) (adminReconcileResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := adminReconcileResult{}
	nowValue := now.UTC().Format(time.RFC3339)
	deployments := make([]*adminDeploymentJob, 0, len(s.deployments))
	for _, deployment := range s.deployments {
		if deployment.Status == "queued" || deployment.Status == "running" {
			deployments = append(deployments, deployment)
		}
	}
	slices.SortFunc(deployments, func(a, b *adminDeploymentJob) int { return a.ID - b.ID })

	for _, deployment := range deployments {
		result.ProcessedJobs++
		deployment.Status = "running"

		for teamID, challengeInstances := range s.instances {
			instance := challengeInstances[deployment.ChallengeID]
			if instance == nil || instance.DeploymentJobID != deployment.ID || instance.RuntimeStatus != "queued" {
				continue
			}
			instance.RuntimeStatus = "ready"
			instance.UpdatedAt = nowValue
			deployment.QueuedTeamCount--
			deployment.ReadyTeamCount++
			result.ProcessedInstances++

			if state := s.teamStates[teamID][deployment.ChallengeID]; state != nil {
				state.Status = "stable"
				state.Checker = "passing"
				state.SSHHint = "solve service to generate SSH credential"
				state.LastEvent = "baseline deployed by controller reconcile"
				state.ResetCooldown = "ready"
			}
		}

		if deployment.QueuedTeamCount < 0 {
			deployment.QueuedTeamCount = 0
		}
		deployment.Status = "completed"
		deployment.CompletedAt = nowValue
		result.CompletedJobs++

		if challenge := s.challenges[deployment.ChallengeID]; challenge != nil {
			challenge.DeployedTeams = s.deployedTeamsForChallengeLocked(deployment.ChallengeID)
			challenge.ReadyTeams = s.readyTeamsForChallengeLocked(deployment.ChallengeID)
			challenge.QueuedTeams = s.queuedTeamsForChallengeLocked(deployment.ChallengeID)
			challenge.RuntimeStatus = challengeRuntimeStatus(challenge.Published, len(s.teams), challenge.ReadyTeams, challenge.QueuedTeams)
		}
	}

	// Handle standalone queued instances (e.g. from new team join)
	for teamID, challengeInstances := range s.instances {
		for challengeID, instance := range challengeInstances {
			if instance.DeploymentJobID == 0 && instance.RuntimeStatus == "queued" {
				instance.RuntimeStatus = "ready"
				instance.UpdatedAt = nowValue
				result.ProcessedInstances++

				if state := s.teamStates[teamID][challengeID]; state != nil {
					state.Status = "stable"
					state.Checker = "passing"
					state.SSHHint = "solve service to generate SSH credential"
					state.LastEvent = "baseline deployed by controller reconcile"
					state.ResetCooldown = "ready"
				}

				if challenge := s.challenges[challengeID]; challenge != nil {
					challenge.DeployedTeams = s.deployedTeamsForChallengeLocked(challengeID)
					challenge.ReadyTeams = s.readyTeamsForChallengeLocked(challengeID)
					challenge.QueuedTeams = s.queuedTeamsForChallengeLocked(challengeID)
					challenge.RuntimeStatus = challengeRuntimeStatus(challenge.Published, len(s.teams), challenge.ReadyTeams, challenge.QueuedTeams)
				}
			}
		}
	}

	return result, nil
}

func (s *memoryStore) DeleteAdminPlayer(_ context.Context, playerID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.players[playerID]; !ok {
		return ErrPlayerNotFound
	}
	delete(s.players, playerID)
	return nil
}

func (s *memoryStore) DeleteAdminTeam(_ context.Context, teamID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	team, ok := s.teams[teamID]
	if !ok {
		return ErrTeamNotFound
	}
	delete(s.teams, teamID)
	delete(s.teamNames, teamID)
	delete(s.teamStates, teamID)
	delete(s.instances, teamID)

	for id, record := range s.players {
		if record.Player.TeamID == teamID {
			delete(s.players, id)
		}
	}

	for i, row := range s.scoreboard {
		if row.Team == team.Name {
			s.scoreboard = append(s.scoreboard[:i], s.scoreboard[i+1:]...)
			break
		}
	}
	return nil
}

func (s *memoryStore) DeleteAdminChallenge(_ context.Context, challengeID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.challenges[challengeID]; !ok {
		return ErrChallengeNotFound
	}
	delete(s.challenges, challengeID)

	for teamID := range s.teamStates {
		delete(s.teamStates[teamID], challengeID)
	}
	for teamID := range s.instances {
		delete(s.instances[teamID], challengeID)
	}
	return nil
}

func (s *memoryStore) SaveAdminChallengeValidation(_ context.Context, result ChallengeValidationResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	challenge, ok := s.challenges[result.ChallengeID]
	if !ok {
		return ErrChallengeNotFound
	}
	validation := result
	challenge.LastValidation = &validation
	return nil
}

func (s *memoryStore) UpdateAdminTeam(_ context.Context, teamID int, input adminUpdateTeamRequest) (adminTeam, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	team, ok := s.teams[teamID]
	if !ok {
		return adminTeam{}, ErrTeamNotFound
	}
	if name := strings.TrimSpace(input.Name); name != "" {
		team.Name = name
		s.teamNames[teamID] = name
	}
	if email := strings.TrimSpace(input.ContactEmail); email != "" {
		team.ContactEmail = strings.ToLower(email)
	}
	s.teams[teamID] = team
	return *team, nil
}

func (s *memoryStore) UpdateAdminPlayer(_ context.Context, playerID int, input adminUpdatePlayerRequest) (adminPlayer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.players[playerID]
	if !ok {
		return adminPlayer{}, ErrPlayerNotFound
	}
	if name := strings.TrimSpace(input.DisplayName); name != "" {
		record.Player.DisplayName = name
	}
	if email := strings.TrimSpace(input.Email); email != "" {
		record.Player.Email = strings.ToLower(email)
	}
	record.Player.Role = normalizedRole(input.Role)
	s.players[playerID] = record
	return record.Player, nil
}

func (s *memoryStore) UpdateAdminChallenge(_ context.Context, challengeID int, input adminUpdateChallengeRequest) (adminChallenge, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, ok := s.challenges[challengeID]
	if !ok {
		return adminChallenge{}, ErrChallengeNotFound
	}
	if name := strings.TrimSpace(input.Name); name != "" {
		challenge.Name = name
	}
	if img := strings.TrimSpace(input.BaselineImage); img != "" {
		challenge.BaselineImage = img
	}
	if img := strings.TrimSpace(input.CheckerImage); img != "" {
		challenge.CheckerImage = img
	}
	challenge.SourceBundlePath = sanitizeSourceBundlePath(input.SourceBundlePath)
	if input.EgressEnabled != nil {
		challenge.EgressEnabled = *input.EgressEnabled
	}
	challenge.LastValidation = nil
	s.challenges[challengeID] = challenge
	return *challenge, nil
}

func (s *memoryStore) Close() error {
	return nil
}

func (s *memoryStore) lookupTeamStateLocked(teamID, challengeID int) (*serviceState, error) {
	challengeStates, ok := s.teamStates[teamID]
	if !ok {
		return nil, ErrChallengeNotFound
	}
	state, ok := challengeStates[challengeID]
	if !ok {
		return nil, ErrChallengeNotFound
	}
	return state, nil
}

func (s *memoryStore) prependAttackEvent(event attackEvent) {
	s.attackFeed = append([]attackEvent{event}, s.attackFeed...)
	if len(s.attackFeed) > 12 {
		s.attackFeed = s.attackFeed[:12]
	}
}

func cloneServiceState(state *serviceState) *serviceState {
	if state == nil {
		return nil
	}
	clone := *state
	return &clone
}

func (s *memoryStore) challengeName(challengeID int) string {
	if challenge, ok := s.challenges[challengeID]; ok {
		return challenge.Name
	}
	return fmt.Sprintf("challenge-%d", challengeID)
}

func (s *memoryStore) playerCountForTeamLocked(teamID int) int {
	count := 0
	for _, player := range s.players {
		if player.Player.TeamID == teamID {
			count++
		}
	}
	return count
}

func (s *memoryStore) deployedTeamsForChallengeLocked(challengeID int) int {
	count := 0
	for _, instances := range s.instances {
		if _, ok := instances[challengeID]; ok {
			count++
		}
	}
	return count
}

func (s *memoryStore) readyTeamsForChallengeLocked(challengeID int) int {
	count := 0
	for _, instances := range s.instances {
		if instance, ok := instances[challengeID]; ok && instance.RuntimeStatus == "ready" {
			count++
		}
	}
	return count
}

func (s *memoryStore) queuedTeamsForChallengeLocked(challengeID int) int {
	count := 0
	for _, instances := range s.instances {
		if instance, ok := instances[challengeID]; ok && instance.RuntimeStatus == "queued" {
			count++
		}
	}
	return count
}

func teamNameForID(teamNames map[int]string, teamID int) string {
	if name, ok := teamNames[teamID]; ok {
		return name
	}
	return fmt.Sprintf("Team %d", teamID)
}

func mustNewAdminPlayerRecord(id, teamID int, teamName, displayName, email, role, password string, createdAt time.Time) *adminPlayerRecord {
	wireGuardPeer := wireguardPeerName(teamID, id)
	wireGuardState, err := newWireGuardPeerState(id, teamID, teamName, displayName, wireGuardPeer, createdAt)
	if err != nil {
		panic(err)
	}

	return &adminPlayerRecord{
		Player: adminPlayer{
			ID:                 id,
			TeamID:             teamID,
			TeamName:           teamName,
			DisplayName:        displayName,
			Email:              email,
			Role:               role,
			WireGuardPeer:      wireGuardPeer,
			WireGuardAddress:   wireGuardState.Address,
			WireGuardStatus:    wireGuardState.Status,
			WireGuardIssuedAt:  wireGuardState.IssuedAt,
			WireGuardRevokedAt: wireGuardState.RevokedAt,
			CreatedAt:          createdAt.UTC().Format(time.RFC3339),
		},
		PasswordHash: mustHashPassword(password),
		WireGuard:    wireGuardState,
	}
}
