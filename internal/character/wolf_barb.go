package character

import (
	"log/slog"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/d2go/pkg/data/state"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
)

const (
	WolfBarbMaxAttacksLoop = 10
)

type WolfBarb struct {
	BaseCharacter
}

func (s *WolfBarb) ShouldIgnoreMonster(m data.Monster) bool {
	return false
}

func (s *WolfBarb) CheckKeyBindings() []skill.ID {
	requireKeybindings := []skill.ID{skill.BattleCommand, skill.BattleOrders, skill.Shout, skill.Werewolf, skill.FeralRage}

	missingKeybindings := []skill.ID{}

	for _, cskill := range requireKeybindings {
		if _, found := s.Data.KeyBindings.KeyBindingForSkill(cskill); !found {
			missingKeybindings = append(missingKeybindings, cskill)
		}
	}

	if len(missingKeybindings) > 0 {
		s.Logger.Debug("There are missing required key bindings.", slog.Any("Bindings", missingKeybindings))
	}

	return missingKeybindings
}

func (s *WolfBarb) KillMonsterSequence(
	monsterSelector func(d game.Data) (data.UnitID, bool),
	skipOnImmunities []stat.Resist,
) error {
	completedAttackLoops := 0
	previousUnitID := 0

	var lastWerewolfCheck time.Time

	for {
		context.Get().PauseIfNotPriority()
		if time.Since(lastWerewolfCheck) > 5*time.Second {
			if !s.Data.PlayerUnit.States.HasState(state.Wolf) {
				if werewolfKB, found := s.Data.KeyBindings.KeyBindingForSkill(skill.Werewolf); found {
					ctx := context.Get()
					ctx.Logger.Debug("Werewolf state missing in combat, rebuffing...")
					ctx.HID.PressKeyBinding(werewolfKB)
					time.Sleep(180 * time.Millisecond)
					ctx.HID.Click(game.RightButton, 640, 340)
					time.Sleep(100 * time.Millisecond)
					ctx.RefreshGameData()
				}
			}
			lastWerewolfCheck = time.Now()
		}

		var id data.UnitID
		var found bool

		id, found = monsterSelector(*s.Data)
		if !found {
			return nil
		}

		if previousUnitID != int(id) {
			completedAttackLoops = 0
		}

		if !s.preBattleChecks(id, skipOnImmunities) {
			return nil
		}

		if completedAttackLoops >= WolfBarbMaxAttacksLoop {
			return nil
		}

		monster, found := s.Data.Monsters.FindByID(id)
		if !found {
			continue
		}

		if monster.Stats[stat.Life] <= 0 {
			continue
		}

		s.castFeralRage(id)

		completedAttackLoops++
		previousUnitID = int(id)
		time.Sleep(time.Millisecond * 100)
	}
}

func (s *WolfBarb) castFeralRage(id data.UnitID) {
	if _, found := s.Data.KeyBindings.KeyBindingForSkill(skill.FeralRage); found {
		step.SecondaryAttack(skill.FeralRage, id, 1, step.Distance(1, 3))
		return
	}
	step.PrimaryAttack(id, 1, false, step.Distance(1, 3))
}

func (s *WolfBarb) BuffSkills() []skill.ID {
	skillsList := make([]skill.ID, 0)
	if _, found := s.Data.KeyBindings.KeyBindingForSkill(skill.BattleCommand); found {
		skillsList = append(skillsList, skill.BattleCommand)
	}
	if _, found := s.Data.KeyBindings.KeyBindingForSkill(skill.Shout); found {
		skillsList = append(skillsList, skill.Shout)
	}
	if _, found := s.Data.KeyBindings.KeyBindingForSkill(skill.BattleOrders); found {
		skillsList = append(skillsList, skill.BattleOrders)
	}
	if _, found := s.Data.KeyBindings.KeyBindingForSkill(skill.Werewolf); found {
		skillsList = append(skillsList, skill.Werewolf)
	}
	return skillsList
}

func (s *WolfBarb) PreCTABuffSkills() []skill.ID {
	return []skill.ID{}
}

func (s *WolfBarb) killMonster(npc npc.ID, t data.MonsterType) error {
	return s.KillMonsterSequence(func(d game.Data) (data.UnitID, bool) {
		m, found := d.Monsters.FindOne(npc, t)
		if !found {
			return 0, false
		}
		return m.UnitID, true
	}, nil)
}

func (s *WolfBarb) KillCountess() error {
	return s.killMonster(npc.DarkStalker, data.MonsterTypeSuperUnique)
}

func (s *WolfBarb) KillAndariel() error {
	return s.killMonster(npc.Andariel, data.MonsterTypeUnique)
}

func (s *WolfBarb) KillSummoner() error {
	return s.killMonster(npc.Summoner, data.MonsterTypeUnique)
}

func (s *WolfBarb) KillDuriel() error {
	return s.killMonster(npc.Duriel, data.MonsterTypeUnique)
}

func (s *WolfBarb) KillMephisto() error {
	return s.killMonster(npc.Mephisto, data.MonsterTypeUnique)
}

func (s *WolfBarb) KillDiablo() error {
	timeout := time.Second * 20
	startTime := time.Now()
	diabloFound := false

	for {
		if time.Since(startTime) > timeout && !diabloFound {
			s.Logger.Error("Diablo was not found, timeout reached")
			return nil
		}

		diablo, found := s.Data.Monsters.FindOne(npc.Diablo, data.MonsterTypeUnique)
		if !found || diablo.Stats[stat.Life] <= 0 {
			if diabloFound {
				return nil
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}

		diabloFound = true
		return s.killMonster(npc.Diablo, data.MonsterTypeUnique)
	}
}

func (s *WolfBarb) KillCouncil() error {
	err := s.killAllCouncilMembers()
	if err != nil {
		return err
	}

	context.Get().EnableItemPickup()
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (s *WolfBarb) killAllCouncilMembers() error {
	context.Get().DisableItemPickup()
	for {
		if !s.anyCouncilMemberAlive() {
			return nil
		}

		err := s.KillMonsterSequence(func(d game.Data) (data.UnitID, bool) {
			for _, m := range d.Monsters.Enemies() {
				if (m.Name == npc.CouncilMember || m.Name == npc.CouncilMember2 || m.Name == npc.CouncilMember3) && m.Stats[stat.Life] > 0 {
					return m.UnitID, true
				}
			}
			return 0, false
		}, nil)

		if err != nil {
			return err
		}
	}
}

func (s *WolfBarb) anyCouncilMemberAlive() bool {
	for _, m := range s.Data.Monsters.Enemies() {
		if (m.Name == npc.CouncilMember || m.Name == npc.CouncilMember2 || m.Name == npc.CouncilMember3) && m.Stats[stat.Life] > 0 {
			return true
		}
	}
	return false
}

func (s *WolfBarb) KillIzual() error {
	return s.killMonster(npc.Izual, data.MonsterTypeUnique)
}

func (s *WolfBarb) KillPindle() error {
	return s.killMonster(npc.DefiledWarrior, data.MonsterTypeSuperUnique)
}

func (s *WolfBarb) KillNihlathak() error {
	return s.killMonster(npc.Nihlathak, data.MonsterTypeSuperUnique)
}

func (s *WolfBarb) KillBaal() error {
	return s.killMonster(npc.BaalCrab, data.MonsterTypeUnique)
}
