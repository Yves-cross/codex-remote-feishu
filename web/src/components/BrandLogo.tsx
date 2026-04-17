const brandLogoHref = "/branding/codex-remote-logo.svg";

export function BrandLogo(props: { className?: string }) {
  const { className } = props;
  return <img className={className} src={brandLogoHref} alt="" aria-hidden="true" />;
}
