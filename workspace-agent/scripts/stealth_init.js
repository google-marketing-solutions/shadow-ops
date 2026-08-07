/**
 * @fileoverview Initialization script to mask automated browser flags.
 */
(() => {
  try {
    Object.defineProperty(navigator, 'webdriver', {
      get: () => undefined,
    });
  } catch (ignored) {
    // Ignore property definition errors on restricted platforms.
  }

  try {
    if (!window.chrome) {
      Object.defineProperty(window, 'chrome', {
        value: {
          runtime: {},
          app: {},
        },
        configurable: true,
      });
    }
  } catch (ignored) {
    // Ignore runtime override failures.
  }

  try {
    Object.defineProperty(navigator, 'languages', {
      get: () => ['en-US', 'en'],
    });
  } catch (ignored) {
    // Ignore navigator overrides if sealed.
  }

  try {
    Object.defineProperty(navigator, 'plugins', {
      get: () => [1, 2, 3, 4, 5],
    });
  } catch (ignored) {
    // Ignore plugin length mock errors.
  }

  try {
    const originalQuery = window.navigator.permissions?.query;
    if (originalQuery) {
      window.navigator.permissions.query = (parameters) => {
        if (parameters && parameters.name === 'notifications') {
          return Promise.resolve({
            state: Notification.permission,
          });
        }
        return originalQuery.call(window.navigator.permissions, parameters);
      };
    }
  } catch (ignored) {
    // Ignore permission hook failure.
  }

})();
