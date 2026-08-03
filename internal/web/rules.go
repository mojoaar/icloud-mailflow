package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/mojoaar/icloud-mailflow/internal/db"
)

func rulesListHandler(repo *db.RulesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rules, err := repo.List()
		if err != nil {
			rules = []db.Rule{}
		}
		data := map[string]any{"Rules": rules, "Search": r.URL.Query().Get("q")}
		if r.Header.Get("HX-Request") == "true" {
			renderPartial(w, "rules_list", data)
			return
		}
		renderPage(w, r, "Rules", "rules_list", data)
	}
}

func rulesNewHandler(foldersRepo *db.FoldersRepo, contactsRepo *db.ContactsRepo, settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		folders, _ := foldersRepo.List()
		contacts, _ := contactsRepo.ListAll()
		tz, _ := settingsRepo.Get("timezone")
		data := map[string]any{"Rule": &db.Rule{Enabled: true}, "New": true, "Fields": conditionFields(), "Folders": folders, "Contacts": contacts, "CondOperator": "OR", "ScheduleDays": []string{}, "ScheduleStart": "", "ScheduleEnd": "", "Timezone": tz}
		renderPage(w, r, "New Rule", "rules_form", data)
	}
}

func rulesCreateHandler(repo *db.RulesRepo, settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		rule := &db.Rule{
			Name:        r.FormValue("name"),
			Description: r.FormValue("description"),
			Enabled:     r.FormValue("enabled") == "on",
		}
		rule.Priority, _ = strconv.Atoi(r.FormValue("priority"))
		if rule.Priority == 0 {
			rules, _ := repo.List()
			for _, r := range rules {
				if r.Name != "_catch_all" {
					rule.Priority++
				}
			}
		}
		rule.ScheduleDays = strings.Join(r.Form["schedule_days"], ",")
		rule.ScheduleStart = r.FormValue("schedule_start")
		rule.ScheduleEnd = r.FormValue("schedule_end")
		parseConditions(r, rule)
		parseActions(r, rule)
		if err := repo.Create(rule); err != nil {
			tz, _ := settingsRepo.Get("timezone")
			renderPage(w, r, "New Rule", "rules_form", map[string]any{"Rule": rule, "Error": "Failed to create rule", "New": true, "Fields": conditionFields(), "Folders": []db.Folder{}, "Contacts": []db.Contact{}, "CondOperator": "OR", "ScheduleDays": []string{}, "ScheduleStart": rule.ScheduleStart, "ScheduleEnd": rule.ScheduleEnd, "Timezone": tz})
			return
		}
		repo.EnsureCatchAll()
		http.Redirect(w, r, "/rules", http.StatusSeeOther)
	}
}

func rulesEditHandler(repo *db.RulesRepo, foldersRepo *db.FoldersRepo, contactsRepo *db.ContactsRepo, settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		rule, err := repo.Get(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		folders, _ := foldersRepo.List()
		contacts, _ := contactsRepo.ListAll()
		op := "AND"
		if len(rule.Groups) > 0 {
			op = rule.Groups[0].Operator
		}
		tz, _ := settingsRepo.Get("timezone")
		renderPage(w, r, "Edit Rule", "rules_form", map[string]any{"Rule": rule, "Edit": true, "Fields": conditionFields(), "Folders": folders, "Contacts": contacts, "CondOperator": op, "ScheduleDays": strings.Split(rule.ScheduleDays, ","), "ScheduleStart": rule.ScheduleStart, "ScheduleEnd": rule.ScheduleEnd, "Timezone": tz})
	}
}

func rulesUpdateHandler(repo *db.RulesRepo, settingsRepo *db.SettingsRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		r.ParseForm()
		rule, err := repo.Get(id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		rule.Name = r.FormValue("name")
		rule.Description = r.FormValue("description")
		rule.Enabled = r.FormValue("enabled") == "on"
		rule.Priority, _ = strconv.Atoi(r.FormValue("priority"))
		rule.ScheduleDays = strings.Join(r.Form["schedule_days"], ",")
		rule.ScheduleStart = r.FormValue("schedule_start")
		rule.ScheduleEnd = r.FormValue("schedule_end")
		parseConditions(r, rule)
		parseActions(r, rule)
		if err := repo.Update(rule); err != nil {
			op := "AND"
			if len(rule.Groups) > 0 {
				op = rule.Groups[0].Operator
			}
			tz, _ := settingsRepo.Get("timezone")
			renderPage(w, r, "Edit Rule", "rules_form", map[string]any{"Rule": rule, "Error": "Failed to update rule", "Edit": true, "Fields": conditionFields(), "Folders": []db.Folder{}, "Contacts": []db.Contact{}, "CondOperator": op, "ScheduleDays": strings.Split(rule.ScheduleDays, ","), "ScheduleStart": rule.ScheduleStart, "ScheduleEnd": rule.ScheduleEnd, "Timezone": tz})
			return
		}
		repo.EnsureCatchAll()
		http.Redirect(w, r, "/rules", http.StatusSeeOther)
	}
}

func rulesDeleteHandler(repo *db.RulesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			if r.Header.Get("HX-Request") == "true" {
				renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "invalid rule id"})
				return
			}
			http.Error(w, "invalid rule id", http.StatusBadRequest)
			return
		}
		if err := repo.Delete(int64(id)); err != nil {
			if r.Header.Get("HX-Request") == "true" {
				renderPartial(w, "toast", map[string]string{"Type": "error", "Message": "Failed to delete rule"})
				return
			}
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		if r.Header.Get("HX-Request") == "true" {
			rules, _ := repo.List()
			renderPartial(w, "rules_list", map[string]any{"Rules": rules})
			return
		}
		http.Redirect(w, r, "/rules", http.StatusSeeOther)
	}
}

func rulesReorderHandler(repo *db.RulesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		var ids []int64
		for _, v := range r.Form["rule_ids"] {
			id, _ := strconv.ParseInt(v, 10, 64)
			ids = append(ids, id)
		}
		repo.Reorder(ids)
		rulesListHandler(repo)(w, r)
	}
}

func rulesReorderMoveHandler(repo *db.RulesRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		id, _ := strconv.ParseInt(r.FormValue("rule_id"), 10, 64)
		dir := r.FormValue("direction")
		rules, _ := repo.List()
		var ids []int64
		idx := -1
		for i, rl := range rules {
			ids = append(ids, rl.ID)
			if rl.ID == id {
				idx = i
			}
		}
		if idx >= 0 {
			if dir == "up" && idx > 0 {
				ids[idx], ids[idx-1] = ids[idx-1], ids[idx]
			}
			if dir == "down" && idx < len(ids)-1 {
				ids[idx], ids[idx+1] = ids[idx+1], ids[idx]
			}
			repo.Reorder(ids)
		}
		rulesListHandler(repo)(w, r)
	}
}

func conditionFields() []map[string]string {
	return []map[string]string{
		{"value": "from", "label": "From"},
		{"value": "to", "label": "To"},
		{"value": "cc", "label": "CC"},
		{"value": "subject", "label": "Subject"},
		{"value": "body", "label": "Body"},
		{"value": "has_attachment", "label": "Has Attachment"},
		{"value": "content_type", "label": "Content Type"},
		{"value": "header", "label": "Header:"},
	}
}

func parseConditions(r *http.Request, rule *db.Rule) {
	rule.Groups = nil
	r.ParseForm()
	fields := r.Form["cond_field"]
	ops := r.Form["cond_op"]
	vals := r.Form["cond_value"]
	if len(fields) == 0 {
		return
	}
	operator := r.FormValue("cond_operator")
	if operator == "" {
		operator = "OR"
	}
	group := db.ConditionGroup{Operator: operator}
	headerNames := r.Form["cond_header_name"]
	for i := range fields {
		if i < len(ops) && i < len(vals) {
			field := fields[i]
			if field == "header" && i < len(headerNames) && headerNames[i] != "" {
				field = "header:" + headerNames[i]
			}
			group.Conditions = append(group.Conditions, db.Condition{
				Field:    field,
				Operator: ops[i],
				Value:    vals[i],
			})
		}
	}
	rule.Groups = []db.ConditionGroup{group}
}

func parseActions(r *http.Request, rule *db.Rule) {
	rule.Actions = nil
	r.ParseForm()
	types := r.Form["action_type"]
	values := r.Form["action_value"]
	for i := range types {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		rule.Actions = append(rule.Actions, db.Action{Type: types[i], Value: v})
	}
}
