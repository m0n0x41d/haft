import { useEffect, useState, useRef } from "react";
import { searchArtifacts, type ArtifactSummary } from "../lib/api";

type NavigateFn = (page: string, id?: string) => void;

export function SearchOverlay({
  open,
  onClose,
  onNavigate,
}: {
  open: boolean;
  onClose: () => void;
  onNavigate: NavigateFn;
}) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<ArtifactSummary[]>([]);
  const [selectedIdx, setSelectedIdx] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open) {
      setQuery("");
      setResults([]);
      setSelectedIdx(0);
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [open]);

  useEffect(() => {
    if (!query.trim()) {
      setResults([]);
      return;
    }
    const timer = setTimeout(() => {
      searchArtifacts(query).then((r) => {
        setResults(r);
        setSelectedIdx(0);
      });
    }, 200);
    return () => clearTimeout(timer);
  }, [query]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") {
      onClose();
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIdx((i) => Math.min(i + 1, results.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIdx((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter" && results[selectedIdx]) {
      handleSelect(results[selectedIdx]);
    }
  };

  const handleSelect = (item: ArtifactSummary) => {
    const pageMap: Record<string, string> = {
      ProblemCard: "problems",
      SolutionPortfolio: "portfolios",
      DecisionRecord: "decisions",
      Note: "dashboard",
    };
    const page = pageMap[item.kind] || "dashboard";
    onNavigate(page, item.id);
    onClose();
  };

  if (!open) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-start justify-center pt-[15vh] z-50" onClick={onClose}>
      <div
        className="bg-surface-1 rounded-xl border border-border w-[560px] shadow-2xl overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Search input */}
        <div className="flex items-center px-4 border-b border-border">
          <span className="text-text-muted mr-2 text-sm">Q</span>
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Search problems, decisions, notes..."
            className="flex-1 py-3 bg-transparent text-sm text-text-primary focus:outline-none"
          />
          <span className="text-xs text-text-muted px-2 py-0.5 rounded bg-surface-2 border border-border">
            esc
          </span>
        </div>

        {/* Results */}
        <div className="max-h-80 overflow-y-auto">
          {results.length === 0 && query.trim() && (
            <p className="text-sm text-text-muted text-center py-6">No results</p>
          )}
          {results.map((item, i) => (
            <button
              key={item.id}
              onClick={() => handleSelect(item)}
              className={`w-full text-left px-4 py-2.5 flex items-center justify-between transition-colors ${
                i === selectedIdx ? "bg-accent/10" : "hover:bg-surface-2"
              }`}
            >
              <div className="flex items-center gap-3 min-w-0">
                <KindBadge kind={item.kind} />
                <span className="text-sm truncate">{item.title}</span>
              </div>
              <span className="text-xs text-text-muted font-mono shrink-0 ml-2">
                {item.id}
              </span>
            </button>
          ))}
        </div>

        {/* Footer */}
        {results.length > 0 && (
          <div className="px-4 py-2 border-t border-border flex items-center gap-4 text-xs text-text-muted">
            <span>up/down navigate</span>
            <span>enter open</span>
            <span>esc close</span>
          </div>
        )}
      </div>
    </div>
  );
}

function KindBadge({ kind }: { kind: string }) {
  const styles: Record<string, string> = {
    ProblemCard: "bg-warning/10 text-warning",
    SolutionPortfolio: "bg-purple-500/10 text-purple-400",
    DecisionRecord: "bg-success/10 text-success",
    Note: "bg-surface-2 text-text-muted",
  };
  const labels: Record<string, string> = {
    ProblemCard: "PROB",
    SolutionPortfolio: "SOL",
    DecisionRecord: "DEC",
    Note: "NOTE",
  };
  return (
    <span
      className={`text-[10px] px-1.5 py-0.5 rounded font-mono ${styles[kind] ?? "bg-surface-2 text-text-muted"}`}
    >
      {labels[kind] ?? kind.slice(0, 4).toUpperCase()}
    </span>
  );
}
