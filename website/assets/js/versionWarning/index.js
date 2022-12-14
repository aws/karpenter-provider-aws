export function versionWarning() {
  const viewingVersion = window.location.pathname.split('/')[1]

  // only alert if we recognize the version string
  if (!viewingVersion || !viewingVersion.match(/^v\d+\.\d+\.\d+$/)) {
    return
  }

  const latestVersion = [
    ...document.querySelectorAll("#navbarVersionSelector .dropdown-item")
  ].map(el => el.dataset.docsVersion)[0]

  if (viewingVersion !== latestVersion) {
    // keep the current path if user is viewing and old version of the docs
    const newPath = window.location.pathname.replace(viewingVersion, latestVersion).replace(/^\//g, '')
    const alertElement = document.createElement('div')
    alertElement.classList.add('alert', 'alert-warning')
    alertElement.innerHTML = `
      <h3>You are viewing Karpenter's <strong>${viewingVersion}</strong> documentation</h3>
      <p>
        Karpenter <strong>${viewingVersion}</strong> is not the latest stable release. 
        For up-to-date documentation, see the <a href="/${newPath}">latest version</a>.
      </p>
    `
    document.querySelector('.td-content').prepend(alertElement)
  }
}
