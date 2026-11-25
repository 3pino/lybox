// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

package com.tailscale.ipn.ui.view

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.tooling.preview.Preview
import androidx.lifecycle.viewmodel.compose.viewModel
import com.tailscale.ipn.App
import com.tailscale.ipn.BuildConfig
import com.tailscale.ipn.R
import com.tailscale.ipn.mdm.AlwaysNeverUserDecides
import com.tailscale.ipn.mdm.MDMSettings
import com.tailscale.ipn.mdm.ShowHide
import com.tailscale.ipn.ui.Links
import com.tailscale.ipn.ui.components.Header
import com.tailscale.ipn.ui.components.ListItemSwitch
import com.tailscale.ipn.ui.components.ListItemText
import com.tailscale.ipn.ui.theme.link
import com.tailscale.ipn.ui.util.AndroidTVUtil
import com.tailscale.ipn.ui.util.AndroidTVUtil.isAndroidTV
import com.tailscale.ipn.ui.util.AppVersion
import com.tailscale.ipn.ui.util.Lists
import com.tailscale.ipn.ui.util.set
import com.tailscale.ipn.ui.viewModel.AppViewModel
import com.tailscale.ipn.ui.viewModel.SettingsNav
import com.tailscale.ipn.ui.viewModel.SettingsViewModel
import java.io.File

@Composable
fun SettingsView(
    settingsNav: SettingsNav,
    viewModel: SettingsViewModel = viewModel(),
    appViewModel: AppViewModel = viewModel(),
) {
  val handler = LocalUriHandler.current

  val user by viewModel.loggedInUser.collectAsState()
  val isAdmin by viewModel.isAdmin.collectAsState()
  val managedByOrganization by viewModel.managedByOrganization.collectAsState()
  val tailnetLockEnabled by viewModel.tailNetLockEnabled.collectAsState()
  val corpDNSEnabled by viewModel.corpDNSEnabled.collectAsState()
  val isVPNPrepared by appViewModel.vpnPrepared.collectAsState()
  val showTailnetLock by MDMSettings.manageTailnetLock.flow.collectAsState()
  val useTailscaleSubnets by MDMSettings.useTailscaleSubnets.flow.collectAsState()
  val isClientRemoteLoggingEnabled by viewModel.isClientRemoteLoggingEnabled.collectAsState()
  var showDisableLoggingDialog by remember { mutableStateOf(false) }
  var lybox by remember { mutableStateOf(LyBoxSettings.load()) }

  Scaffold(
      topBar = {
        Header(
            titleRes = R.string.settings_title,
            onBack = settingsNav.onNavigateBackHome,
            isTv = isAndroidTV(),
        )
      }
  ) { innerPadding ->
    Column(modifier = Modifier.padding(innerPadding).verticalScroll(rememberScrollState())) {
      if (isVPNPrepared) {
        UserView(
            profile = user,
            isTv = isAndroidTV(),
            actionState = UserActionState.NAV,
            onClick = settingsNav.onNavigateToUserSwitcher,
        )
      }

      if (isAdmin && !isAndroidTV()) {
        Lists.ItemDivider()
        AdminTextView { handler.openUri(Links.ADMIN_URL) }
      }

      Lists.SectionDivider()
      ListItemText(
          R.string.dns_settings,
          subtitle =
              corpDNSEnabled?.let {
                stringResource(
                    if (it) R.string.using_tailscale_dns else R.string.not_using_tailscale_dns
                )
              },
          onClick = settingsNav.onNavigateToDNSSettings,
      )

      Lists.ItemDivider()
      ListItemText(
          R.string.split_tunneling,
          subtitle = stringResource(R.string.filter_apps_allowed_to_access_tailscale),
          onClick = settingsNav.onNavigateToSplitTunneling,
      )

      if (showTailnetLock.value == ShowHide.Show) {
        Lists.ItemDivider()
        ListItemText(
            R.string.tailnet_lock,
            subtitle =
                tailnetLockEnabled?.let {
                  stringResource(if (it) R.string.enabled else R.string.disabled)
                },
            onClick = settingsNav.onNavigateToTailnetLock,
        )
      }
      if (useTailscaleSubnets.value == AlwaysNeverUserDecides.UserDecides) {
        Lists.ItemDivider()
        ListItemText(R.string.subnet_routing, onClick = settingsNav.onNavigateToSubnetRouting)
      }

      Lists.ItemDivider()
      ListItemSwitch(
          R.string.client_remote_logging_enabled,
          subtitle =
              stringResource(
                  if (MDMSettings.isMDMConfigured)
                      R.string.client_remote_logging_enabled_subtitle_mdm
                  else R.string.client_remote_logging_enabled_subtitle
              ),
          isOn = isClientRemoteLoggingEnabled,
          enabled = !MDMSettings.isMDMConfigured,
          onToggle = {
            if (isClientRemoteLoggingEnabled) {
              showDisableLoggingDialog = true
            } else {
              viewModel.toggleIsClientRemoteLoggingEnabled()
            }
          },
      )

      if (!AndroidTVUtil.isAndroidTV()) {
        Lists.ItemDivider()
        ListItemText(R.string.permissions, onClick = settingsNav.onNavigateToPermissions)
      }

      managedByOrganization.value?.let {
        Lists.ItemDivider()
        ListItemText(
            title = stringResource(R.string.managed_by_orgName, it),
            onClick = settingsNav.onNavigateToManagedBy,
        )
      }

      Lists.SectionDivider()
      Lists.MutedHeader(text = stringResource(R.string.lybox_section))
      ListItemSwitch(
          R.string.lybox_cellular_derp,
          subtitle = stringResource(R.string.lybox_cellular_derp_subtitle),
          isOn = lybox.cellularDerp,
          onToggle = { lybox = lybox.copy(cellularDerp = it).also(LyBoxSettings::save) },
      )
      Lists.ItemDivider()
      ListItemSwitch(
          R.string.lybox_relaxed_probing,
          subtitle = stringResource(R.string.lybox_relaxed_probing_subtitle),
          isOn = lybox.relaxedProbing,
          onToggle = { lybox = lybox.copy(relaxedProbing = it).also(LyBoxSettings::save) },
      )
      Lists.ItemDivider()
      ListItemSwitch(
          R.string.lybox_netcheck_cache,
          subtitle = stringResource(R.string.lybox_netcheck_cache_subtitle),
          isOn = lybox.netcheckCache,
          onToggle = { lybox = lybox.copy(netcheckCache = it).also(LyBoxSettings::save) },
      )
      Lists.InfoItem(stringResource(R.string.lybox_section_info))

      Lists.SectionDivider()
      ListItemText(R.string.bug_report, onClick = settingsNav.onNavigateToBugReport)

      Lists.ItemDivider()
      ListItemText(
          R.string.about_tailscale,
          subtitle = "${stringResource(id = R.string.version)} ${AppVersion.Short()}",
          onClick = settingsNav.onNavigateToAbout,
      )

      // TODO: put a heading for the debug section
      if (BuildConfig.DEBUG) {
        Lists.SectionDivider()
        Lists.MutedHeader(text = stringResource(R.string.internal_debug_options))
        ListItemText(R.string.mdm_settings, onClick = settingsNav.onNavigateToMDMSettings)
      }
    }
  }

  if (showDisableLoggingDialog) {
    AlertDialog(
        onDismissRequest = { showDisableLoggingDialog = false },
        title = { Text(stringResource(R.string.client_remote_logging_disable_confirm_title)) },
        text = { Text(stringResource(R.string.client_remote_logging_disable_confirm_message)) },
        confirmButton = {
          TextButton(
              onClick = {
                showDisableLoggingDialog = false
                viewModel.toggleIsClientRemoteLoggingEnabled()
              }
          ) {
            Text(
                stringResource(R.string.client_remote_logging_disable_confirm_button),
                color = MaterialTheme.colorScheme.error,
            )
          }
        },
        dismissButton = {
          TextButton(onClick = { showDisableLoggingDialog = false }) {
            Text(stringResource(R.string.cancel))
          }
        },
    )
  }
}

/**
 * LyBox feature toggles. Stored as key=0|1 lines in files/lybox_settings, which the Go backend
 * (libtailscale/power.go) re-reads every few seconds. All on is the LyBox default; all off behaves
 * like stock Tailscale.
 */
data class LyBoxSettings(
    val cellularDerp: Boolean = true,
    val relaxedProbing: Boolean = true,
    val netcheckCache: Boolean = true,
) {
  companion object {
    private fun file() = File(App.get().filesDir, "lybox_settings")

    fun load(): LyBoxSettings {
      val values = runCatching {
        file().readLines().mapNotNull {
          it.split("=", limit = 2)
              .takeIf { p -> p.size == 2 }
              ?.let { p -> p[0].trim() to (p[1].trim() != "0") }
        }
      }
          .getOrDefault(emptyList())
          .toMap()
      return LyBoxSettings(
          cellularDerp = values["cellular_derp"] ?: true,
          relaxedProbing = values["relaxed_probing"] ?: true,
          netcheckCache = values["netcheck_cache"] ?: true,
      )
    }

    fun save(s: LyBoxSettings) {
      fun b(v: Boolean) = if (v) 1 else 0
      file()
          .writeText(
              "cellular_derp=${b(s.cellularDerp)}\n" +
                  "relaxed_probing=${b(s.relaxedProbing)}\n" +
                  "netcheck_cache=${b(s.netcheckCache)}\n"
          )
    }
  }
}

@Composable
fun AdminTextView(onNavigateToAdminConsole: () -> Unit) {
  val adminStr = buildAnnotatedString {
    append(stringResource(id = R.string.settings_admin_prefix))

    pushStringAnnotation(tag = "link", annotation = Links.ADMIN_URL)
    withStyle(
        style =
            SpanStyle(
                color = MaterialTheme.colorScheme.link,
                textDecoration = TextDecoration.Underline,
            )
    ) {
      append(stringResource(id = R.string.settings_admin_link))
    }
  }

  Lists.InfoItem(adminStr, onClick = onNavigateToAdminConsole)
}

@Preview
@Composable
fun SettingsPreview() {
  val vm = SettingsViewModel()
  vm.corpDNSEnabled.set(true)
  vm.tailNetLockEnabled.set(true)
  vm.isAdmin.set(true)
  vm.managedByOrganization.set("Tails and Scales Inc.")
  SettingsView(SettingsNav({}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}), vm)
}
