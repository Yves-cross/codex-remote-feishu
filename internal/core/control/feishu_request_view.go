package control

// FeishuRequestView is the UI-owned request payload carried across the
// control->adapter boundary. The final Feishu card renderer may still project
// it into a retained adapter-local DTO shape.
type FeishuRequestView = FeishuDirectRequestPrompt
