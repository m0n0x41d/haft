import { useEffect, useState } from "react";
import { listDecisions, getDecision, type DecisionSummary, type DecisionDetail } from "../lib/api";

type NavigateFn = (page: "dashboard" | "problems" | "decisions", id?: string) => void;

export function Decisions({
  selectedId,
  onNavigate: _onNavigate,
}: {
  selectedId: string | null;
  onNavigate: NavigateFn;
}) {
  const [decisions, setDecisions] = useState<DecisionSummary[]>([]);
  const [detail, setDetail] = useState<DecisionDetail | null>(null);
  const [activeId, setActiveId] = useState<string | null>(selectedId);

  useEffect(() => {
    listDecisions().then(setDecisions).catch(console.error);
  }, []);

  useEffect(() => {
    if (!activeId) {
      setDetail(null);
      return;
    }
    getDecision(activeId).then(setDetail).catch(console.error);
  }, [activeId]);

  return (
    <div className="flex gap-6 h-[calc(100vh-7rem)]">
      <div className="w-80 shrink-0 overflow-y-auto space-y-1">
        {decisions.map((d) => (
          <button
            key={d.id}
            onClick={() => setActiveId(d.id)}
            className={`w-full text-left px-4 py-3 rounded-lg transition-colors border ${
              activeId === d.id
                ? "bg-surface-2 border-accent/30"
                : "bg-surface-1 border-transparent hover:bg-surface-2 hover:border-border"
            }`}
          >
            <span className="text-sm font-medium block truncate">{d.selected_title}</span>
            <div className="flex items-center gap-2 mt-1">
              <span className="text-xs text-text-muted font-mono">{d.id}</span>
              {d.valid_until && <span className="text-xs text-text-muted">until {d.valid_until}</span>}
            </div>
            {d.weakest_link && <p className="text-xs text-warning/70 mt-1 line-clamp-1">WLNK: {d.weakest_link}</p>}
          </button>
        ))}
        {decisions.length === 0 && <p className="text-sm text-text-muted text-center py-8">No decisions</p>}
      </div>

      <div className="flex-1 overflow-y-auto">
        {detail ? (
          <DecisionDetailPanel detail={detail} />
        ) : activeId ? (
          <p className="text-sm text-text-muted py-8 text-center">Loading...</p>
        ) : (
          <p className="text-sm text-text-muted py-8 text-center">Select a decision to view details</p>
        )}
      </div>
    </div>
  );
}

function DecisionDetailPanel({ detail }: { detail: DecisionDetail }) {
  const [implementing, setImplementing] = useState(false);
  const [verifying, setVerifying] = useState(false);

  const handleImplement = async () => {
    setImplementing(true);
    try {
      const bindingPath = "../../wailsjs/go/main/App";
      const mod = await import(/* @vite-ignore */ bindingPath);
      if (mod.ImplementDecision) {
        await mod.ImplementDecision(detail.id, "claude", true, "");
      }
    } catch (e) {
      console.error("Implement failed:", e);
    } finally {
      setImplementing(false);
    }
  };

  const handleVerify = async () => {
    setVerifying(true);
    try {
      const bindingPath = "../../wailsjs/go/main/App";
      const mod = await import(/* @vite-ignore */ bindingPath);
      if (mod.VerifyDecision) {
        await mod.VerifyDecision(detail.id, "claude");
      }
    } catch (e) {
      console.error("Verify failed:", e);
    } finally {
      setVerifying(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <div>
          <h2 className="text-xl font-semibold">{detail.selected_title}</h2>
          <p className="text-xs text-text-muted font-mono mt-1">{detail.id}</p>
          {detail.valid_until && <p className="text-xs text-text-secondary mt-1">Valid until {detail.valid_until}</p>}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <button
            onClick={handleVerify}
            disabled={verifying}
            className="text-xs px-3 py-1.5 rounded-lg bg-surface-2 text-text-secondary hover:bg-surface-3 border border-border transition-colors disabled:opacity-50"
          >
            {verifying ? "Verifying..." : "Verify Claims"}
          </button>
          <button
            onClick={handleImplement}
            disabled={implementing}
            className="text-xs px-3 py-1.5 rounded-lg bg-accent text-white hover:bg-accent-hover transition-colors disabled:opacity-50"
          >
            {implementing ? "Spawning..." : "Implement"}
          </button>
        </div>
      </div>

      <Field label="Why Selected" value={detail.why_selected} />
      {detail.selection_policy && <Field label="Selection Policy" value={detail.selection_policy} />}
      {detail.weakest_link && <Field label="Weakest Link" value={detail.weakest_link} variant="warning" />}
      {detail.counterargument && <Field label="Counterargument" value={detail.counterargument} />}

      {detail.invariants?.length > 0 && <ListField label="Invariants (must hold)" items={detail.invariants} />}
      {detail.admissibility?.length > 0 && <ListField label="Not Acceptable" items={detail.admissibility} variant="danger" />}

      {detail.claims?.length > 0 && (
        <div>
          <h4 className="text-xs text-text-muted uppercase tracking-wider mb-2">Claims & Predictions</h4>
          <div className="border border-border rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-surface-2">
                  <th className="text-left px-4 py-2 text-xs text-text-muted font-medium">Claim</th>
                  <th className="text-left px-4 py-2 text-xs text-text-muted font-medium">Observable</th>
                  <th className="text-left px-4 py-2 text-xs text-text-muted font-medium">Threshold</th>
                  <th className="text-left px-4 py-2 text-xs text-text-muted font-medium">Status</th>
                </tr>
              </thead>
              <tbody>
                {detail.claims.map((c) => (
                  <tr key={c.id} className="border-t border-border">
                    <td className="px-4 py-2">{c.claim}</td>
                    <td className="px-4 py-2 text-text-secondary">{c.observable}</td>
                    <td className="px-4 py-2 font-mono text-xs">{c.threshold}</td>
                    <td className="px-4 py-2"><ClaimStatusBadge status={c.status} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {detail.why_not_others?.length > 0 && (
        <div>
          <h4 className="text-xs text-text-muted uppercase tracking-wider mb-2">Rejected Alternatives</h4>
          <div className="space-y-2">
            {detail.why_not_others.map((r, i) => (
              <div key={i} className="bg-surface-1 rounded-lg px-4 py-3 border border-border">
                <span className="text-sm font-medium text-text-secondary">{r.variant}</span>
                <p className="text-xs text-text-muted mt-1">{r.reason}</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {detail.pre_conditions?.length > 0 && <ListField label="Pre-conditions" items={detail.pre_conditions} />}
      {detail.post_conditions?.length > 0 && <ListField label="Post-conditions" items={detail.post_conditions} />}
      {detail.evidence_requirements?.length > 0 && <ListField label="Evidence Requirements" items={detail.evidence_requirements} />}

      {detail.rollback_triggers?.length > 0 && (
        <div className="bg-danger/5 rounded-lg p-4 border border-danger/20">
          <h4 className="text-xs text-danger uppercase tracking-wider mb-2">Rollback Plan</h4>
          <div className="space-y-2">
            <div>
              <span className="text-xs text-text-muted">Triggers:</span>
              <ul className="mt-1 space-y-1">
                {detail.rollback_triggers.map((t, i) => <li key={i} className="text-sm text-text-secondary">{t}</li>)}
              </ul>
            </div>
            {detail.rollback_steps?.length > 0 && (
              <div>
                <span className="text-xs text-text-muted">Steps:</span>
                <ol className="mt-1 space-y-1 list-decimal list-inside">
                  {detail.rollback_steps.map((s, i) => <li key={i} className="text-sm text-text-secondary">{s}</li>)}
                </ol>
              </div>
            )}
            {detail.rollback_blast_radius && <p className="text-xs text-text-muted mt-2">Blast radius: {detail.rollback_blast_radius}</p>}
          </div>
        </div>
      )}

      {detail.refresh_triggers?.length > 0 && <ListField label="Refresh Triggers" items={detail.refresh_triggers} />}
    </div>
  );
}

function Field({ label, value, variant }: { label: string; value: string; variant?: "warning" | "danger" }) {
  const borderColor = variant === "warning" ? "border-warning/20" : variant === "danger" ? "border-danger/20" : "border-border";
  return (
    <div>
      <h4 className="text-xs text-text-muted uppercase tracking-wider mb-1">{label}</h4>
      <p className={`text-sm text-text-primary bg-surface-1 rounded-lg px-4 py-3 border ${borderColor}`}>{value}</p>
    </div>
  );
}

function ListField({ label, items, variant }: { label: string; items: string[]; variant?: "danger" }) {
  return (
    <div>
      <h4 className="text-xs text-text-muted uppercase tracking-wider mb-1">{label}</h4>
      <ul className="space-y-1">
        {items.map((item, i) => (
          <li key={i} className={`text-sm bg-surface-1 rounded-lg px-4 py-2 border ${variant === "danger" ? "border-danger/20 text-danger/80" : "border-border text-text-primary"}`}>{item}</li>
        ))}
      </ul>
    </div>
  );
}

function ClaimStatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    unverified: "bg-surface-2 text-text-muted",
    supported: "bg-success/10 text-success",
    weakened: "bg-warning/10 text-warning",
    refuted: "bg-danger/10 text-danger",
  };
  return <span className={`text-xs px-2 py-0.5 rounded-full ${styles[status] ?? styles.unverified}`}>{status || "unverified"}</span>;
}
