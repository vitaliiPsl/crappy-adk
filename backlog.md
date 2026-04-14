001. ~~restructure. already extracted agent into the package of it's own, but kit package is still a mess~~
002. rethink the dependency on the model config. at the moment it is used to decided when to perform auto compaction by comparing usage of the last model call with the model input limit.
003. need a way to control the cost of agent run. thinking max number of turns, max usage, max tools and max cost (again, dependency on the model config).
004. cluade code lately have been calling the same tool over and over again in a loop. need to add a kill switch that would detect this and stop execution. or signal this to model
005. consider making agent stateful and extractint flow/runner
006. anthropic just released a new feature called advisor strategy https://x.com/i/status/2042308622181339453 that turns more powerful model into an advisor for low cost models bringing cost per task down. So when low cost model can't make a decision it calls powerful model which reads the shared context and offers and advice / plan. Prob need to look in a direction of multi model / agent collaboration in general. claude code has support for this through teams and tasks
007. convert html response to markdow in web fetch tool
008. consider moving ~~thinking~~(moved) and tool call/result to the content part
009. consider embedding model events into agent events, and agent events into stream events reducing duplication
