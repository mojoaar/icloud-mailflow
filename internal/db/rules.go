package db

import (
	"database/sql"
)

type Rule struct {
	ID          int64            `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Priority    int              `json:"priority"`
	Enabled     bool             `json:"enabled"`
	CreatedAt   string           `json:"created_at"`
	UpdatedAt   string           `json:"updated_at"`
	Groups      []ConditionGroup `json:"groups,omitempty"`
	Actions     []Action         `json:"actions,omitempty"`
}

type ConditionGroup struct {
	ID         int64            `json:"id"`
	RuleID     int64            `json:"rule_id"`
	ParentID   *int64           `json:"parent_id"`
	Operator   string           `json:"logic_operator"`
	Conditions []Condition      `json:"conditions,omitempty"`
	Groups     []ConditionGroup `json:"groups,omitempty"`
}

type Condition struct {
	ID       int64  `json:"id"`
	GroupID  int64  `json:"group_id"`
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type Action struct {
	ID     int64  `json:"id"`
	RuleID int64  `json:"rule_id"`
	Type   string `json:"type"`
	Value  string `json:"value"`
}

type RulesRepo struct{ DB *sql.DB }

func NewRulesRepo(d *sql.DB) *RulesRepo { return &RulesRepo{DB: d} }

func (r *RulesRepo) List() ([]Rule, error) {
	rows, err := r.DB.Query(`SELECT id, name, description, priority, enabled, created_at, updated_at FROM rules ORDER BY priority ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Rule
	for rows.Next() {
		var rule Rule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Description, &rule.Priority, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := r.loadConditions(&out[i]); err != nil {
			return nil, err
		}
		if err := r.loadActions(&out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *RulesRepo) Get(id int64) (*Rule, error) {
	var rule Rule
	err := r.DB.QueryRow(`SELECT id, name, description, priority, enabled, created_at, updated_at FROM rules WHERE id = ?`, id).
		Scan(&rule.ID, &rule.Name, &rule.Description, &rule.Priority, &rule.Enabled, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := r.loadConditions(&rule); err != nil {
		return nil, err
	}
	if err := r.loadActions(&rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *RulesRepo) Create(rule *Rule) error {
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO rules (name, description, priority, enabled) VALUES (?, ?, ?, ?)`,
		rule.Name, rule.Description, rule.Priority, rule.Enabled)
	if err != nil {
		return err
	}
	rule.ID, _ = res.LastInsertId()
	if err := r.saveConditions(tx, rule); err != nil {
		return err
	}
	if err := r.saveActions(tx, rule); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *RulesRepo) Update(rule *Rule) error {
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE rules SET name=?, description=?, priority=?, enabled=?, updated_at=datetime('now') WHERE id=?`,
		rule.Name, rule.Description, rule.Priority, rule.Enabled, rule.ID)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM conditions WHERE group_id IN (SELECT id FROM condition_groups WHERE rule_id=?)`, rule.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM condition_groups WHERE rule_id=?`, rule.ID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM actions WHERE rule_id=?`, rule.ID); err != nil {
		return err
	}
	if err := r.saveConditions(tx, rule); err != nil {
		return err
	}
	if err := r.saveActions(tx, rule); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *RulesRepo) Delete(id int64) error {
	_, err := r.DB.Exec(`DELETE FROM rules WHERE id=?`, id)
	return err
}

func (r *RulesRepo) Reorder(ids []int64) error {
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for pri, id := range ids {
		if _, err := tx.Exec(`UPDATE rules SET priority=? WHERE id=?`, pri, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *RulesRepo) loadConditions(rule *Rule) error {
	rows, err := r.DB.Query(`SELECT id, rule_id, parent_id, logic_operator FROM condition_groups WHERE rule_id=? ORDER BY id`, rule.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	groups := []ConditionGroup{}
	for rows.Next() {
		var g ConditionGroup
		var parentID sql.NullInt64
		if err := rows.Scan(&g.ID, &g.RuleID, &parentID, &g.Operator); err != nil {
			return err
		}
		if parentID.Valid {
			g.ParentID = &parentID.Int64
		}
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	condRows, err := r.DB.Query(`SELECT id, group_id, field, operator, value FROM conditions WHERE group_id IN (SELECT id FROM condition_groups WHERE rule_id=?)`, rule.ID)
	if err != nil {
		return err
	}
	defer condRows.Close()
	condMap := map[int64][]Condition{}
	for condRows.Next() {
		var c Condition
		if err := condRows.Scan(&c.ID, &c.GroupID, &c.Field, &c.Operator, &c.Value); err != nil {
			return err
		}
		condMap[c.GroupID] = append(condMap[c.GroupID], c)
	}
	if err := condRows.Err(); err != nil {
		return err
	}

	for i := range groups {
		groups[i].Conditions = condMap[groups[i].ID]
	}

	rule.Groups = buildGroupTree(groups, nil)
	return nil
}

func buildGroupTree(all []ConditionGroup, parentID *int64) []ConditionGroup {
	var out []ConditionGroup
	for _, g := range all {
		if (g.ParentID == nil && parentID == nil) || (g.ParentID != nil && parentID != nil && *g.ParentID == *parentID) {
			g.Groups = buildGroupTree(all, &g.ID)
			out = append(out, g)
		}
	}
	return out
}

func (r *RulesRepo) loadActions(rule *Rule) error {
	rows, err := r.DB.Query(`SELECT id, rule_id, type, value FROM actions WHERE rule_id=? ORDER BY id`, rule.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var a Action
		if err := rows.Scan(&a.ID, &a.RuleID, &a.Type, &a.Value); err != nil {
			return err
		}
		rule.Actions = append(rule.Actions, a)
	}
	return rows.Err()
}

func (r *RulesRepo) saveConditions(tx *sql.Tx, rule *Rule) error {
	var saveGroup func(g ConditionGroup, parentID *int64) error
	saveGroup = func(g ConditionGroup, parentID *int64) error {
		res, err := tx.Exec(`INSERT INTO condition_groups (rule_id, parent_id, logic_operator) VALUES (?, ?, ?)`,
			rule.ID, parentID, g.Operator)
		if err != nil {
			return err
		}
		gID, _ := res.LastInsertId()
		for _, c := range g.Conditions {
			if _, err := tx.Exec(`INSERT INTO conditions (group_id, field, operator, value) VALUES (?, ?, ?, ?)`,
				gID, c.Field, c.Operator, c.Value); err != nil {
				return err
			}
		}
		for _, sub := range g.Groups {
			if err := saveGroup(sub, &gID); err != nil {
				return err
			}
		}
		return nil
	}
	for _, g := range rule.Groups {
		if err := saveGroup(g, nil); err != nil {
			return err
		}
	}
	return nil
}

func (r *RulesRepo) saveActions(tx *sql.Tx, rule *Rule) error {
	for _, a := range rule.Actions {
		if _, err := tx.Exec(`INSERT INTO actions (rule_id, type, value) VALUES (?, ?, ?)`,
			rule.ID, a.Type, a.Value); err != nil {
			return err
		}
	}
	return nil
}

func (r *RulesRepo) EnsureCatchAll() error {
	var count int
	if err := r.DB.QueryRow(`SELECT COUNT(*) FROM rules WHERE name = ?`, "_catch_all").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		r.DB.Exec(`DELETE FROM conditions WHERE group_id IN (SELECT id FROM condition_groups WHERE rule_id = (SELECT id FROM rules WHERE name = ?))`, "_catch_all")
		r.DB.Exec(`DELETE FROM condition_groups WHERE rule_id = (SELECT id FROM rules WHERE name = ?)`, "_catch_all")
		return nil
	}
	var maxPri sql.NullInt64
	r.DB.QueryRow(`SELECT MAX(priority) FROM rules`).Scan(&maxPri)
	pri := 999
	if maxPri.Valid {
		pri = int(maxPri.Int64) + 1
	}
	res, err := r.DB.Exec(`INSERT INTO rules (name, description, priority, enabled) VALUES (?, ?, ?, 1)`,
		"_catch_all", "Built-in catch-all — moves unmatched mail to Inbox", pri)
	if err != nil {
		return err
	}
	ruleID, _ := res.LastInsertId()
	r.DB.Exec(`INSERT INTO actions (rule_id, type, value) VALUES (?, 'move_to_folder', 'INBOX')`, ruleID)
	return nil
}
