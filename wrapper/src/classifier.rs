use serde_json::Value;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MessageClassification {
    AgentMessage,
    ToolCall,
    ServerRequest,
    TurnLifecycle,
    ThreadLifecycle,
    Unknown,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ClassifiedMessage {
    pub classification: MessageClassification,
    pub method: Option<String>,
    pub thread_id: Option<String>,
    pub turn_id: Option<String>,
}

#[derive(Debug, Default, Clone, PartialEq, Eq)]
pub struct MessageClassifier {
    current_thread_id: Option<String>,
    current_turn_id: Option<String>,
}

impl MessageClassifier {
    pub fn classify(&mut self, line: &[u8]) -> ClassifiedMessage {
        let parsed = serde_json::from_slice::<Value>(line).ok();
        let method = parsed
            .as_ref()
            .and_then(|value| get_string(value, &[&["method"]]))
            .map(str::to_owned);
        let item_type = parsed
            .as_ref()
            .and_then(|value| get_string(value, &[&["params", "item", "type"], &["params", "itemType"]]))
            .map(str::to_owned);

        if let Some(value) = parsed.as_ref() {
            self.update_context(value, method.as_deref());
        }

        ClassifiedMessage {
            classification: classify_method(method.as_deref(), item_type.as_deref()),
            method,
            thread_id: self.current_thread_id.clone(),
            turn_id: self.current_turn_id.clone(),
        }
    }

    #[cfg(test)]
    pub fn current_thread_id(&self) -> Option<&str> {
        self.current_thread_id.as_deref()
    }

    #[cfg(test)]
    pub fn current_turn_id(&self) -> Option<&str> {
        self.current_turn_id.as_deref()
    }

    fn update_context(&mut self, value: &Value, method: Option<&str>) {
        match method {
            Some("thread/started") => {
                if let Some(thread_id) = get_string(
                    value,
                    &[&["params", "threadId"], &["threadId"], &["params", "thread", "id"]],
                ) {
                    self.current_thread_id = Some(thread_id.to_owned());
                }
                self.current_turn_id = None;
            }
            Some("turn/started") => {
                if let Some(thread_id) = get_string(
                    value,
                    &[&["params", "threadId"], &["threadId"], &["params", "thread", "id"]],
                ) {
                    self.current_thread_id = Some(thread_id.to_owned());
                }

                if let Some(turn_id) =
                    get_string(value, &[&["params", "turnId"], &["turnId"], &["params", "turn", "id"]])
                {
                    self.current_turn_id = Some(turn_id.to_owned());
                }
            }
            _ => {}
        }
    }
}

fn classify_method(method: Option<&str>, item_type: Option<&str>) -> MessageClassification {
    match method {
        Some("item/agentMessage/delta") => MessageClassification::AgentMessage,
        Some("item/started" | "item/completed")
            if matches!(
                item_type,
                Some("commandExecution" | "fileChange" | "dynamicToolCall")
            ) =>
        {
            MessageClassification::ToolCall
        }
        Some(method) if method.starts_with("serverRequest/") => MessageClassification::ServerRequest,
        Some("turn/started" | "turn/completed") => MessageClassification::TurnLifecycle,
        Some("thread/started") => MessageClassification::ThreadLifecycle,
        _ => MessageClassification::Unknown,
    }
}

fn get_string<'a>(value: &'a Value, paths: &[&[&str]]) -> Option<&'a str> {
    paths.iter().find_map(|path| get_string_at_path(value, path))
}

fn get_string_at_path<'a>(value: &'a Value, path: &[&str]) -> Option<&'a str> {
    let mut current = value;
    for segment in path {
        current = current.get(*segment)?;
    }
    current.as_str()
}

#[cfg(test)]
mod tests {
    use super::{MessageClassification, MessageClassifier};

    #[test]
    fn classifies_agent_message_delta() {
        let mut classifier = MessageClassifier::default();

        let classified =
            classifier.classify(br#"{"method":"item/agentMessage/delta","params":{"delta":"hi"}}"#);

        assert_eq!(classified.classification, MessageClassification::AgentMessage);
        assert_eq!(classified.method.as_deref(), Some("item/agentMessage/delta"));
    }

    #[test]
    fn classifies_command_execution_item_started_as_tool_call() {
        let mut classifier = MessageClassifier::default();

        let classified = classifier.classify(
            br#"{"method":"item/started","params":{"item":{"type":"commandExecution"}}}"#,
        );

        assert_eq!(classified.classification, MessageClassification::ToolCall);
    }

    #[test]
    fn classifies_file_change_item_completed_as_tool_call() {
        let mut classifier = MessageClassifier::default();

        let classified = classifier.classify(
            br#"{"method":"item/completed","params":{"item":{"type":"fileChange"}}}"#,
        );

        assert_eq!(classified.classification, MessageClassification::ToolCall);
    }

    #[test]
    fn classifies_dynamic_tool_call_item_completed_as_tool_call() {
        let mut classifier = MessageClassifier::default();

        let classified = classifier.classify(
            br#"{"method":"item/completed","params":{"item":{"type":"dynamicToolCall"}}}"#,
        );

        assert_eq!(classified.classification, MessageClassification::ToolCall);
    }

    #[test]
    fn classifies_server_request_prefix() {
        let mut classifier = MessageClassifier::default();

        let classified =
            classifier.classify(br#"{"method":"serverRequest/approval","params":{"id":"req-1"}}"#);

        assert_eq!(classified.classification, MessageClassification::ServerRequest);
    }

    #[test]
    fn classifies_turn_lifecycle_and_tracks_ids() {
        let mut classifier = MessageClassifier::default();

        let classified = classifier.classify(
            br#"{"method":"turn/started","params":{"turnId":"turn-1","threadId":"thread-1"}}"#,
        );

        assert_eq!(classified.classification, MessageClassification::TurnLifecycle);
        assert_eq!(classified.thread_id.as_deref(), Some("thread-1"));
        assert_eq!(classified.turn_id.as_deref(), Some("turn-1"));
        assert_eq!(classifier.current_thread_id(), Some("thread-1"));
        assert_eq!(classifier.current_turn_id(), Some("turn-1"));
    }

    #[test]
    fn classifies_thread_lifecycle_and_resets_turn_context() {
        let mut classifier = MessageClassifier::default();
        classifier.classify(
            br#"{"method":"turn/started","params":{"turnId":"turn-1","threadId":"thread-1"}}"#,
        );

        let classified =
            classifier.classify(br#"{"method":"thread/started","params":{"threadId":"thread-2"}}"#);

        assert_eq!(classified.classification, MessageClassification::ThreadLifecycle);
        assert_eq!(classified.thread_id.as_deref(), Some("thread-2"));
        assert_eq!(classified.turn_id, None);
        assert_eq!(classifier.current_thread_id(), Some("thread-2"));
        assert_eq!(classifier.current_turn_id(), None);
    }

    #[test]
    fn uses_tracked_ids_for_following_messages() {
        let mut classifier = MessageClassifier::default();
        classifier.classify(
            br#"{"method":"thread/started","params":{"threadId":"thread-1"}}"#,
        );
        classifier.classify(
            br#"{"method":"turn/started","params":{"turnId":"turn-1","threadId":"thread-1"}}"#,
        );

        let classified =
            classifier.classify(br#"{"method":"item/agentMessage/delta","params":{"delta":"hello"}}"#);

        assert_eq!(classified.classification, MessageClassification::AgentMessage);
        assert_eq!(classified.thread_id.as_deref(), Some("thread-1"));
        assert_eq!(classified.turn_id.as_deref(), Some("turn-1"));
    }

    #[test]
    fn leaves_non_tool_item_lifecycle_as_unknown() {
        let mut classifier = MessageClassifier::default();

        let classified = classifier
            .classify(br#"{"method":"item/started","params":{"item":{"type":"agentMessage"}}}"#);

        assert_eq!(classified.classification, MessageClassification::Unknown);
    }

    #[test]
    fn malformed_json_is_unknown_and_preserves_existing_context() {
        let mut classifier = MessageClassifier::default();
        classifier.classify(
            br#"{"method":"turn/started","params":{"turnId":"turn-1","threadId":"thread-1"}}"#,
        );

        let classified = classifier.classify(b"{not valid json\n");

        assert_eq!(classified.classification, MessageClassification::Unknown);
        assert_eq!(classified.thread_id.as_deref(), Some("thread-1"));
        assert_eq!(classified.turn_id.as_deref(), Some("turn-1"));
        assert_eq!(classifier.current_thread_id(), Some("thread-1"));
        assert_eq!(classifier.current_turn_id(), Some("turn-1"));
    }

    #[test]
    fn does_not_modify_original_message_bytes() {
        let mut classifier = MessageClassifier::default();
        let original = br#"{"method":"item/agentMessage/delta","params":{"delta":"hello\nworld"}}"#;
        let message = original.to_vec();

        let _ = classifier.classify(&message);

        assert_eq!(message, original);
    }
}
