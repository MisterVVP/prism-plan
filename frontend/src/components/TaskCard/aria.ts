export const aria = {
  root: (title: string) => ({
    'aria-label': title,
    role: 'button',
    'aria-roledescription': 'Task card',
    tabIndex: 0
  })
};
