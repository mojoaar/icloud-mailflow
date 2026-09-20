package config

import (
	"strings"
	"time"
	_ "time/tzdata"
)

func ValidTimezone(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	_, err := time.LoadLocation(name)
	return err == nil
}

func TimezoneSuggestions() []string {
	return []string{
		"UTC",
		"Africa/Accra", "Africa/Algiers", "Africa/Cairo", "Africa/Casablanca",
		"Africa/Johannesburg", "Africa/Lagos", "Africa/Nairobi", "Africa/Tunis",
		"America/Anchorage", "America/Argentina/Buenos_Aires", "America/Asuncion",
		"America/Barbados", "America/Belize", "America/Bogota", "America/Caracas",
		"America/Chicago", "America/Costa_Rica", "America/Denver", "America/Edmonton",
		"America/El_Salvador", "America/Guatemala", "America/Guayaquil", "America/Halifax",
		"America/Havana", "America/Jamaica", "America/La_Paz",
		"America/Lima", "America/Los_Angeles", "America/Managua", "America/Mexico_City",
		"America/Montevideo", "America/Nassau", "America/New_York", "America/Panama",
		"America/Phoenix", "America/Port-au-Prince", "America/Puerto_Rico",
		"America/Regina", "America/Santiago", "America/Santo_Domingo",
		"America/Sao_Paulo", "America/St_Johns", "America/Tegucigalpa",
		"America/Toronto", "America/Vancouver", "America/Winnipeg",
		"Asia/Almaty", "Asia/Baghdad", "Asia/Bangkok", "Asia/Beirut", "Asia/Colombo",
		"Asia/Dhaka", "Asia/Dubai", "Asia/Ho_Chi_Minh", "Asia/Hong_Kong",
		"Asia/Irkutsk", "Asia/Jakarta", "Asia/Jerusalem", "Asia/Karachi",
		"Asia/Kathmandu", "Asia/Kolkata", "Asia/Krasnoyarsk", "Asia/Kuala_Lumpur",
		"Asia/Kuwait", "Asia/Manila", "Asia/Novosibirsk", "Asia/Phnom_Penh",
		"Asia/Qatar", "Asia/Riyadh", "Asia/Seoul", "Asia/Shanghai", "Asia/Singapore",
		"Asia/Taipei", "Asia/Tashkent", "Asia/Tehran", "Asia/Tokyo",
		"Asia/Ulaanbaatar", "Asia/Vladivostok", "Asia/Yangon", "Asia/Yekaterinburg",
		"Atlantic/Azores", "Atlantic/Canary", "Atlantic/Reykjavik",
		"Australia/Adelaide", "Australia/Brisbane", "Australia/Darwin",
		"Australia/Hobart", "Australia/Melbourne", "Australia/Perth",
		"Australia/Sydney",
		"Europe/Amsterdam", "Europe/Athens", "Europe/Belgrade", "Europe/Berlin",
		"Europe/Brussels", "Europe/Bucharest", "Europe/Budapest", "Europe/Copenhagen",
		"Europe/Dublin", "Europe/Helsinki", "Europe/Istanbul", "Europe/Kyiv",
		"Europe/Lisbon", "Europe/London", "Europe/Madrid", "Europe/Minsk",
		"Europe/Moscow", "Europe/Oslo", "Europe/Paris", "Europe/Prague",
		"Europe/Riga", "Europe/Rome", "Europe/Sofia", "Europe/Stockholm",
		"Europe/Tallinn", "Europe/Vienna", "Europe/Vilnius", "Europe/Warsaw",
		"Europe/Zagreb", "Europe/Zurich",
		"Indian/Maldives", "Indian/Mauritius",
		"Pacific/Auckland", "Pacific/Fiji", "Pacific/Guam", "Pacific/Honolulu",
		"Pacific/Port_Moresby", "Pacific/Samoa", "Pacific/Tahiti",
	}
}
