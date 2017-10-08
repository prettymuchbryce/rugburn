function transform (state)
	if state["title"] ~= nil then
		state["title" = string.upper(state["title"])
	end

	return state
end
