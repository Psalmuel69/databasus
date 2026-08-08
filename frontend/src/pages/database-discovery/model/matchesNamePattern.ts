export const matchesNamePattern = (name: string, pattern: string): boolean => {
  const trimmedPattern = pattern.trim();

  if (!trimmedPattern) {
    return false;
  }

  const escapedPattern = trimmedPattern.replace(/[.+^${}()|[\]\\]/g, '\\$&');
  const regexSource = `^${escapedPattern.replace(/\*/g, '.*')}$`;

  return new RegExp(regexSource, 'i').test(name);
};
