import * as React from "react";
import { RotateCcw, AlertTriangle } from "lucide-react";
import { Button } from "@/components/ui/button";

interface ErrorBoundaryProps {
  children: React.ReactNode;
  /** Optional custom fallback. Receives a `reset` callback to retry rendering. */
  fallback?: (reset: () => void) => React.ReactNode;
  /** Optional hook for logging/telemetry. */
  onError?: (error: Error, info: React.ErrorInfo) => void;
}

interface ErrorBoundaryState {
  hasError: boolean;
}

/**
 * App-level error boundary with a friendly, user-facing fallback.
 * Never shows a raw stack trace to the user; details are logged to the console
 * (and to `onError`) only, so developers can still debug.
 */
export class ErrorBoundary extends React.Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false };
    this.reset = this.reset.bind(this);
  }

  static getDerivedStateFromError(): ErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo): void {
    // Surface to console for developers; never render the stack to users.
    console.error("[ErrorBoundary]", error, info.componentStack);
    this.props.onError?.(error, info);
  }

  reset(): void {
    this.setState({ hasError: false });
  }

  render(): React.ReactNode {
    if (!this.state.hasError) {
      return this.props.children;
    }

    if (this.props.fallback) {
      return this.props.fallback(this.reset);
    }

    return (
      <div
        role="alert"
        aria-live="assertive"
        className="flex min-h-[60vh] w-full flex-col items-center justify-center gap-4 p-6 text-center"
      >
        <div className="flex size-14 items-center justify-center rounded-full bg-destructive/10 text-destructive">
          <AlertTriangle className="size-7" aria-hidden="true" />
        </div>
        <div className="space-y-1">
          <h2 className="text-lg font-semibold text-foreground">
            Something went wrong
          </h2>
          <p className="max-w-sm text-sm text-muted-foreground">
            An unexpected error occurred. You can try again, and if the problem
            keeps happening, restart the app.
          </p>
        </div>
        <Button
          onClick={this.reset}
          className="min-h-11 gap-2"
          aria-label="Retry"
        >
          <RotateCcw className="size-4" aria-hidden="true" />
          Try again
        </Button>
      </div>
    );
  }
}

export default ErrorBoundary;
