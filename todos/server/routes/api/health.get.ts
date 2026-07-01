export default defineEventHandler(() => {
  const config = useRuntimeConfig();
  return {
    status: "ok",
    app: config.appName,
    uptime: Math.floor(process.uptime()),
    node: process.version,
  };
});
