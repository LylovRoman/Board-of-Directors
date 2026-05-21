export function LoadingBlock(props: { label: string }) {
  return (
      <div className="loading-block" role="status" aria-live="polite">
        <span className="loader-ring" />
        <strong>{props.label}</strong>
      </div>
  );
}
