use std::path::Path;

pub const READY: &str = "ready";
pub const NEEDS_INIT: &str = "needs_init";
pub const NEEDS_ONBOARD: &str = "needs_onboard";
pub const MISSING: &str = "missing";
pub const CORE_SOURCE: &str = "core";
pub const DEGRADED_SOURCE: &str = "degraded_core_unavailable";

#[derive(Debug, Clone, serde::Deserialize, serde::Serialize, PartialEq, Eq)]
pub struct ProjectReadinessFacts {
    pub status: String,
    pub exists: bool,
    pub has_haft: bool,
    pub has_specs: bool,
    #[serde(default)]
    pub readiness_source: String,
    #[serde(default)]
    pub readiness_error: String,
}

pub fn inspect_project_readiness(project_root: &str) -> ProjectReadinessFacts {
    let root = project_root.trim();

    inspect_project_readiness_with_core(root, inspect_readiness_via_core)
}

pub fn project_is_ready(project_root: &str) -> bool {
    let facts = inspect_project_readiness(project_root);

    facts.status == READY
}

pub fn project_not_ready_message(project_root: &str, facts: &ProjectReadinessFacts) -> String {
    let base = format!(
        "project is not ready: {project_root} status={}",
        facts.status
    );

    if facts.readiness_error.trim().is_empty() {
        return base;
    }

    format!("{base}; {}", facts.readiness_error)
}

fn inspect_project_readiness_with_core(
    root: &str,
    inspect_core: impl Fn(&str) -> Result<ProjectReadinessFacts, String>,
) -> ProjectReadinessFacts {
    match inspect_core(root) {
        Ok(facts) => normalize_core_facts(facts),
        Err(err) => degraded_project_readiness(root, &err),
    }
}

fn inspect_readiness_via_core(project_root: &str) -> Result<ProjectReadinessFacts, String> {
    let payload = serde_json::json!({ "path": project_root });
    let value = crate::rpc::call_rpc("project-readiness", Some(payload), Some(project_root))?;
    let mut facts = serde_json::from_value::<ProjectReadinessFacts>(value)
        .map_err(|err| format!("parse project-readiness response: {err}"))?;

    facts.readiness_source = CORE_SOURCE.into();
    facts.readiness_error.clear();

    Ok(facts)
}

fn normalize_core_facts(mut facts: ProjectReadinessFacts) -> ProjectReadinessFacts {
    facts.readiness_source = CORE_SOURCE.into();
    facts.readiness_error.clear();

    if status_is_known(&facts.status) {
        return facts;
    }

    facts.status = NEEDS_ONBOARD.into();
    facts.has_specs = false;
    facts.readiness_source = DEGRADED_SOURCE.into();
    facts.readiness_error = "core readiness returned an unknown status".into();
    facts
}

fn status_is_known(status: &str) -> bool {
    matches!(status, READY | NEEDS_INIT | NEEDS_ONBOARD | MISSING)
}

fn degraded_project_readiness(root: &str, err: &str) -> ProjectReadinessFacts {
    let root_path = Path::new(root);
    let error = degraded_readiness_error(err);

    if root.is_empty() || !root_path.is_dir() {
        return ProjectReadinessFacts {
            status: MISSING.into(),
            exists: false,
            has_haft: false,
            has_specs: false,
            readiness_source: DEGRADED_SOURCE.into(),
            readiness_error: error,
        };
    }

    let has_haft = root_path.join(".haft/project.yaml").is_file();
    if !has_haft {
        return ProjectReadinessFacts {
            status: NEEDS_INIT.into(),
            exists: true,
            has_haft: false,
            has_specs: false,
            readiness_source: DEGRADED_SOURCE.into(),
            readiness_error: error,
        };
    }

    ProjectReadinessFacts {
        status: NEEDS_ONBOARD.into(),
        exists: true,
        has_haft: true,
        has_specs: false,
        readiness_source: DEGRADED_SOURCE.into(),
        readiness_error: error,
    }
}

fn degraded_readiness_error(err: &str) -> String {
    format!("core readiness unavailable; degraded fallback used: {err}")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn core_readiness_states_are_authoritative() {
        let cases = [
            (MISSING, false, false, false),
            (NEEDS_INIT, true, false, false),
            (NEEDS_ONBOARD, true, true, false),
            (READY, true, true, true),
        ];

        for (status, exists, has_haft, has_specs) in cases {
            let facts = inspect_project_readiness_with_core("/tmp/project", |_| {
                Ok(readiness_facts(status, exists, has_haft, has_specs))
            });

            assert_eq!(facts.status, status);
            assert_eq!(facts.exists, exists);
            assert_eq!(facts.has_haft, has_haft);
            assert_eq!(facts.has_specs, has_specs);
            assert_eq!(facts.readiness_source, CORE_SOURCE);
            assert_eq!(facts.readiness_error, "");
        }
    }

    #[test]
    fn malformed_active_spec_reported_by_core_stays_needs_onboard() {
        let facts = inspect_project_readiness_with_core("/tmp/project", |_| {
            Ok(readiness_facts(NEEDS_ONBOARD, true, true, false))
        });

        assert_eq!(facts.status, NEEDS_ONBOARD);
        assert!(!facts.has_specs);
        assert_ne!(facts.status, READY);
    }

    #[test]
    fn missing_term_map_reported_by_core_stays_needs_onboard() {
        let facts = inspect_project_readiness_with_core("/tmp/project", |_| {
            Ok(readiness_facts(NEEDS_ONBOARD, true, true, false))
        });

        assert_eq!(facts.status, NEEDS_ONBOARD);
        assert!(!facts.has_specs);
        assert_ne!(facts.status, READY);
    }

    #[test]
    fn degraded_fallback_classifies_missing_and_needs_init() {
        let missing = inspect_project_readiness_with_core("/tmp/does-not-exist", |_| {
            Err("haft unavailable".into())
        });
        assert_eq!(missing.status, MISSING);
        assert_eq!(missing.readiness_source, DEGRADED_SOURCE);
        assert!(
            missing
                .readiness_error
                .contains("core readiness unavailable")
        );

        let root = tempfile::tempdir().expect("create temp dir");
        let needs_init =
            inspect_project_readiness_with_core(root.path().to_string_lossy().as_ref(), |_| {
                Err("haft unavailable".into())
            });
        assert_eq!(needs_init.status, NEEDS_INIT);
        assert_eq!(needs_init.readiness_source, DEGRADED_SOURCE);
        assert!(!needs_init.has_specs);
    }

    #[test]
    fn degraded_fallback_never_marks_initialized_project_ready() {
        let root = tempfile::tempdir().expect("create temp dir");
        let haft_dir = root.path().join(".haft");
        std::fs::create_dir_all(&haft_dir).expect("create .haft");
        std::fs::write(haft_dir.join("project.yaml"), "id: qnt_test\nname: test\n")
            .expect("write project config");

        let facts =
            inspect_project_readiness_with_core(root.path().to_string_lossy().as_ref(), |_| {
                Err("haft unavailable".into())
            });

        assert_eq!(facts.status, NEEDS_ONBOARD);
        assert!(facts.exists);
        assert!(facts.has_haft);
        assert!(!facts.has_specs);
        assert_eq!(facts.readiness_source, DEGRADED_SOURCE);
        assert_ne!(facts.status, READY);
    }

    fn readiness_facts(
        status: &str,
        exists: bool,
        has_haft: bool,
        has_specs: bool,
    ) -> ProjectReadinessFacts {
        ProjectReadinessFacts {
            status: status.into(),
            exists,
            has_haft,
            has_specs,
            readiness_source: CORE_SOURCE.into(),
            readiness_error: String::new(),
        }
    }
}
